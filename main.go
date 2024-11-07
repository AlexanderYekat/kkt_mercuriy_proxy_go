package main

import (
	"cto_ksm_mercury/consttypes"
	merc "cto_ksm_mercury/sendtcp"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kardianos/service"
	// Добавляем этот импорт
)

// Config структура для хранения настроек
type Config struct {
	SourcePort               int    `json:"sourcePort"`
	KktEmulation             bool   `json:"kktEmulation"`
	KktIP                    string `json:"kktIP"`
	KktPort                  int    `json:"kktPort"`
	ComPort                  int    `json:"comPort"`
	CountAttemptsOfMarkCheck int    `json:"countAttemptsOfMarkCheck"`
	UserMerc                 int    `json:"userMerc"`
	PasswUserMerc            string `json:"passwUserMerc"`
	PauseOfMarksMistake      int    `json:"pauseOfMarksMistake"`
}

var (
	logger    service.Logger
	config    Config
	mu        sync.RWMutex
	svcConfig = &service.Config{
		Name:         "cto_ksm_mercury",
		DisplayName:  "cto_ksm_mercury",
		Description:  "ЦТО КСМ - для ККТ Меркурий",
		UserName:     "NT AUTHORITY\\NetworkService", // Используем NetworkService для ограниченного доступа
		Dependencies: []string{"Tcpip", "Dnscache"},
		Option: service.KeyValue{
			"StartTimeout": "120",
		},
	}
	fileLogger *log.Logger
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
	config = Config{
		SourcePort:               2579,
		KktEmulation:             false,
		KktIP:                    "localhost",
		KktPort:                  7778,
		ComPort:                  1,
		CountAttemptsOfMarkCheck: 10,
		UserMerc:                 0,
		PasswUserMerc:            "",
		PauseOfMarksMistake:      10,
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

	// Читаем тело запроса
	var doc consttypes.TDocument
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Логируем полученный документ
	if logger != nil {
		logger.Infof("Получен документ с %d позициями", len(doc.Items))
	}

	var err error
	sessionkey := ""
	mercSNODefault := -1
	descrError := ""

	sessionkey, descrError, err = merc.CheckStatsuConnectionKKT(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, "", config.UserMerc, config.PasswUserMerc)
	if err != nil {
		if !config.KktEmulation {
			mercSNODefault = 0
			http.Error(w, "Ошибка подключения к ККТ: "+descrError, http.StatusInternalServerError)
			return
		} else {
			mercSNODefault, err = merc.GetSNOByDefault(config.KktEmulation, config.KktIP, config.KktPort, sessionkey)
			if err != nil {
				http.Error(w, "Ошибка получения SNO: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	} else {
		mercSNODefault, err = merc.GetSNOByDefault(config.KktEmulation, config.KktIP, config.KktPort, sessionkey)
		if err != nil {
			http.Error(w, "Ошибка получения SNO: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	//проверка марок
	merc.BreakAndClearProccessOfMarks(config.KktIP, config.KktPort, config.ComPort, sessionkey, config.UserMerc, config.PasswUserMerc)
	for _, item := range doc.Items {
		if item.Mark == "" {
			continue
		}
		_, err = merc.RunProcessCheckMark(config.KktEmulation, config.KktIP, config.KktPort, config.CountAttemptsOfMarkCheck, config.PauseOfMarksMistake, sessionkey, item.Mark)
		if err != nil {
			http.Error(w, "Ошибка проверки марок: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Здесь будет добавлена логика обработки документа
	answer, err := merc.PrintCheck(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, doc, "", mercSNODefault, false, 0, "", false)
	if err != nil {
		http.Error(w, "Ошибка печати чека: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if sessionkey != "" {
		merc.Closesession(config.KktIP, config.KktPort, &sessionkey)
	}

	var answerJson consttypes.TAnswerMercur
	err = json.Unmarshal([]byte(answer), &answerJson)
	if err != nil {
		http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Временный ответ
	response := map[string]interface{}{
		"status":    "success",
		"message":   "Документ получен",
		"itemCount": len(doc.Items),
		"fiscNumb":  answerJson.FiscalDocNum,
		"fiscSign":  answerJson.FiscalSign,
		//"dateTime": answerJson.
		"answer": answer,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

func initFileLogger() error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = filepath.Join(os.Getenv("USERPROFILE"), "Application Data")
	}

	logDir := filepath.Join(appData, "CTO_KSM", "ProxyFMU", "logs")
	os.MkdirAll(logDir, 0755)

	currentTime := time.Now().Format("2006-01-02")
	logPath := filepath.Join(logDir, "service_"+currentTime+".log")

	// Выводим путь к файлу логов
	fmt.Printf("Путь к файлу логов: %s\n", logPath)

	logFile, err := os.OpenFile(
		logPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}

	fileLogger = log.New(logFile, "", log.LstdFlags)
	return nil
}

func main() {
	fmt.Println("Начинаем инициализацию файла логов...")
	if err := initFileLogger(); err != nil {
		fmt.Printf("Ошибка инициализации файла логов: %v\n", err)
	}

	if runtime.GOOS == "windows" {
		// Установить совместимость с Windows 7
		os.Setenv("GODEBUG", "netdns=go")
	}

	if !service.Interactive() {
		isAdmin, err := isUserAdmin()
		if err != nil || !isAdmin {
			log.Fatal("Программа должна быть запущена с правами администратора")
			return
		}
	}

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
			fmt.Println("Начинаем установку службы...")
			fileLogger.Println("Начинаем установку службы...")
			err = s.Install()
			if err != nil {
				errMsg := fmt.Sprintf("Ошибка установки службы: %v", err)
				fmt.Println(errMsg)
				fileLogger.Println(errMsg)
				logger.Error(errMsg)
				log.Fatal(err)
			}
			fmt.Println("Служба успешно установлена")
			fileLogger.Println("Служба успешно установлена")
			return
		case "uninstall":
			err = s.Uninstall()
			if err != nil {
				log.Fatal("Не удалось удалить службу: ", err)
			}
			fmt.Println("Служба успешно удалена")
			return
		case "start":
			fmt.Println("Начинаем запуск службы...")
			fileLogger.Println("Начинаем запуск службы...")
			err = s.Start()
			if err != nil {
				errMsg := fmt.Sprintf("Ошибка запуска службы: %v", err)
				fmt.Println(errMsg)
				fileLogger.Println(errMsg)
				log.Fatal(err)
			}
			fmt.Println("Служба успешно запущена")
			fileLogger.Println("Служба успешно запущена")
			return
		case "stop":
			fmt.Println("Начинаем остановку службы...")
			fileLogger.Println("Начинаем остановку службы...")
			err = s.Stop()
			if err != nil {
				errMsg := fmt.Sprintf("Ошибка остановки службы: %v", err)
				fmt.Println(errMsg)
				fileLogger.Println(errMsg)
				log.Fatal(err)
			}
			fmt.Println("Служба успешно остановлена")
			fileLogger.Println("Служба успешно остановлена")
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

func isUserAdmin() (bool, error) {
	fmt.Println("Проверяем права администратора...")
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false, err
	}
	return true, nil
}
