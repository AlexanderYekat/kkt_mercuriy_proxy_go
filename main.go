package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/kardianos/service"
)

// Config структура для хранения настроек
type Config struct {
	SourcePort int    `json:"sourcePort"`
	TargetPort int    `json:"targetPort"`
	TargetHost string `json:"targetHost"`
}

var (
	logger    service.Logger
	config    Config
	mu        sync.RWMutex
	svcConfig = &service.Config{
		Name:        "cto_ksm_proxyfmu",
		DisplayName: "cto_ksm_proxyfmu",
		Description: "ЦТО КСМ - прокси-сервис для FMU - разрешительный режим",
		UserName:    "LocalSystem",
		// Исправляем Type на ServiceType
		//ServiceType: service.WindowsService,
		Option: service.KeyValue{
			"StartTimeout": "120",
		},
	}
)

type program struct{}

func (p *program) Start(s service.Service) error {
	// Создаем канал для graceful shutdown
	stop := make(chan struct{})
	go func() {
		p.run()
		close(stop)
	}()

	// Логируем успешный запуск
	if logger != nil {
		logger.Info("Служба успешно запущена")
	}
	return nil
}

func (p *program) Stop(s service.Service) error {
	// Логируем остановку
	if logger != nil {
		logger.Info("Служба останавливается...")
	}
	// Здесь можно добавить cleanup код если необходимо
	return nil
}

func (p *program) run() {
	if logger != nil {
		logger.Info("Инициализация службы...")
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.SourcePort),
		Handler: nil, // использует DefaultServeMux
	}

	setupRoutes()

	if logger != nil {
		logger.Infof("Сервер запущен на порту %d", config.SourcePort)
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		if logger != nil {
			logger.Error(err)
		}
	}
}

// Функция для получения пути к файлу конфигурации
func getConfigPath() string {
	// Получаем путь к Application Data
	appData := os.Getenv("APPDATA")
	if appData == "" {
		// Для Windows XP путь может быть другим
		appData = filepath.Join(os.Getenv("USERPROFILE"), "Application Data")
	}

	// Создаем директорию для нашего приложения, если её нет
	appDir := filepath.Join(appData, "CTO_KSM", "ProxyFMU")
	os.MkdirAll(appDir, 0755)

	// Возвращаем полный путь к файлу конфигурации
	return filepath.Join(appDir, "ctoksm_proxyfmu_config.json")
}

func loadConfig() error {
	// Значения по умолчани
	config = Config{
		SourcePort: 2579,
		TargetPort: 2578,
		TargetHost: "localhost",
	}

	// Попытка загруить конфиг из файла
	configPath := getConfigPath()
	data, err := ioutil.ReadFile(configPath)
	if err == nil {
		err = json.Unmarshal(data, &config)
		if err != nil {
			return err
		}
	}
	return nil
}

func saveConfig() error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(getConfigPath(), data, 0644)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	mu.RLock()
	targetURL := fmt.Sprintf("http://%s:%d/document", config.TargetHost, config.TargetPort)
	mu.RUnlock()

	proxyReq, err := http.NewRequest(http.MethodPost, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Ошибка при создании запроса", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}
	proxyReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Ошибка при отправке запроса", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.Error(err)
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		json.NewEncoder(w).Encode(config)
		mu.RUnlock()

	case http.MethodPost:
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		config = newConfig
		mu.Unlock()

		if err := saveConfig(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Перезапускаем службу
		if service.Interactive() {
			// Если запущено как приложение, сообщаем о необходимости ручного перезапуска
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Settings saved. Please restart the application to apply changes."))
		} else {
			// Ели запущено как служба, пытаемся перезапустить
			s, err := service.New(&program{}, svcConfig)
			if err == nil {
				s.Restart()
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Settings saved. Service will be restarted."))
		}
	}
}

func setupRoutes() {
	// Setup API routes
	http.HandleFunc("/document", handleRequest)
	http.HandleFunc("/api/settings", handleSettings)

	// Setup static files using regular file server
	staticDir := "./static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// If running as service, try to find static directory relative to executable
		exe, err := os.Executable()
		if err == nil {
			staticDir = filepath.Join(filepath.Dir(exe), "static")
		}
	}
	http.Handle("/", http.FileServer(http.Dir(staticDir)))
}

func runAsApplication() {
	fmt.Printf("Запуск в режиме приложения на http://localhost:%d\n", config.SourcePort)
	setupRoutes()
	if err := http.ListenAndServe(fmt.Sprintf(":%d", config.SourcePort), nil); err != nil {
		log.Fatal(err)
	}
}

func main() {
	if err := loadConfig(); err != nil {
		log.Printf("Ошибка загрузки конфига: %v", err)
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal("Ошибка создания службы: ", err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal("Ошибка создания логгера: ", err)
	}

	// Добавляем логирование при запуске
	logger.Info("Инициализация службы...")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			err = s.Install()
			if err != nil {
				log.Fatal("Не удалось установить службу: ", err)
			}
			fmt.Println("Служба успено установлена")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				log.Fatal("Не удалось удалить службу: ", err)
			}
			fmt.Println("Служба успешно удалена")
			return
		case "start":
			err = s.Start()
			if err != nil {
				log.Fatal("Не удалось запустить службу: ", err)
			}
			fmt.Println("Служба запущена")
			return
		case "stop":
			err = s.Stop()
			if err != nil {
				log.Fatal("Не удалось остановить службу: ", err)
			}
			fmt.Println("Служба остановлена")
			return
		case "run":
			runAsApplication()
			return
		}
	}

	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
