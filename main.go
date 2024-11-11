package main

import (
	"bytes"
	"cto_ksm_mercury/consttypes"
	merc "cto_ksm_mercury/sendtcp"
	"encoding/json"
	"fmt"
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
	config    Config
	mu        sync.RWMutex
	svcConfig = &service.Config{
		Name:        "cto_ksm_mercury",
		DisplayName: "cto_ksm_mercury",
		Description: "ЦТО КСМ - для ККТ Меркурий",
		UserName:    "NT AUTHORITY\\LocalService", // Используем LocalService для минимальных прав
		Dependencies: []string{
			"Tcpip",    // Сетевой стек
			"Dnscache", // DNS-кэширование
			"RpcSs",    // Remote Procedure Call (RPC)
			"nsi",      // Network Store Interface Service
			//"EventLog", // Для логирования событий
		},
		Option: service.KeyValue{
			"StartTimeout":   "220",
			"ServiceSIDType": "1", // Ограничиваем SID службы
		},
	}
)

var currentDocument consttypes.TDocument // Глобальная переменная для хранения текущего документа

type program struct{}

func (p *program) Start(s service.Service) error {
	// Создаем канал для graceful shutdown

	ioutil.WriteFile(getLogPath()+"/service_start2.log", []byte("Служба запущена"), 0644)
	ioutil.WriteFile(getLogPath()+"/service_start3.log", []byte(getLogPath()), 0644)
	ioutil.WriteFile(getLogPath()+"/service_start4.log", []byte(getConfigPath()), 0644)
	if err := initLogger(); err != nil {
		errMsg := fmt.Sprintf("Ошибка инициализации логгера: %v", err)
		ioutil.WriteFile(getLogPath()+"/service_start.log", []byte(errMsg), 0644)
		//log.Fatal("Ошибка инициализации логгера:", err)
		//fmt.Println("Ошибка инициализации логгера:", err)
	}

	stop := make(chan struct{})
	go func() {
		p.run()
		close(stop)
	}()

	return nil
}

func (p *program) Stop(s service.Service) error {
	// Здесь можно добавить cleanup код если необходимо
	return nil
}

func (p *program) run() {

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.SourcePort),
		Handler: nil, // использует DefaultServeMux
	}

	setupRoutes()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	}
}

// Функция для получения пути к файлу конфигурации
func getConfigPath() string {
	fmt.Println("Получаем путь к Application Data...")
	// Используем ProgramData вместо APPDATA
	//programData := os.Getenv("ProgramData")
	//userDir, _ := os.UserHomeDir()
	userDir, _ := os.UserHomeDir()
	programData := filepath.Join(userDir, "AppData", "Local")
	appDir := filepath.Join(programData, "cto_ksm_mercury")
	os.MkdirAll(appDir, 0755)
	return filepath.Join(appDir, "ctoksm_proxyfmu_config.json")
}

// Функция для получения пути к файлу конфигурации
func getLogPath() string {
	fmt.Println("Получаем путь к Application Data...")
	// Используем ProgramData вместо APPDATA
	//programData := os.Getenv("ProgramData")
	userDir, _ := os.UserHomeDir()
	ioutil.WriteFile(getConfigPath()+"/service_start.log", []byte("Получаем путь к Application Data..."), 0644)
	programData := filepath.Join(userDir, "AppData", "Local")
	appDir := filepath.Join(programData, "cto_ksm_mercury")
	os.MkdirAll(appDir, 0755)
	//return filepath.Join(appDir, "service.log")
	return filepath.Join(appDir)
}

func loadConfig() error {
	config = Config{
		SourcePort:               2579,
		KktEmulation:             false,
		KktIP:                    "localhost",
		KktPort:                  50009,
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

func saveConfig() (string, error) {
	fmt.Println("Сохраняем конфигурацию...")
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return "", err
	}
	return getConfigPath(), ioutil.WriteFile(getConfigPath(), data, 0644)
}

func initLogger() error {
	logDir := getLogPath()
	// Создаем директорию, если она не существует
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("ошибка создания директории для логов: %v", err)
	}
	logPath := filepath.Join(logDir, "service.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла лога: %v", err)
	}

	consttypes.Logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lshortfile)
	return nil
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Получен новый запрос: %s %s", r.Method, r.URL.Path)
	}

	response := consttypes.TDocumentResponse{
		Success: false,
		Message: "",
		Answer:  consttypes.TAnswerMercur{},
	}

	if r.Method != http.MethodPost {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Метод не разрешен: %s", r.Method)
		}
		response.Message = "Метод не разрешен"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Читаем тело запроса
	var command consttypes.TCommand
	var bodyBytes []byte
	bodyBytes, _ = ioutil.ReadAll(r.Body)
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Тело запроса: %s", bodyBytes)
	}
	// Очищаем от специальных символов
	cleanBody := bytes.Trim(bodyBytes, "\xef\xbb\xbf\x00\x1f") // Удаляем BOM и другие спецсимволы
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Очищенное тело запроса: %s", cleanBody)
	}

	if err := json.NewDecoder(bytes.NewReader(cleanBody)).Decode(&command); err != nil {
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Ошибка разбора JSON: %v", err)
		}
		response.Message = "Ошибка разбора JSON: " + err.Error()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Получен команда: %v", command)
	}

	answer := ""
	mercSNODefault := -1
	var errOfCommand error

	if command.Params.IsTest {
		config.KktEmulation = true

		response.Success = true
		response.Message = "Тестовый режим"
		response.FiscNumb = "123456"
		response.FiscSign = "987654321"
		response.Answer = consttypes.TAnswerMercur{
			Result:      0,
			Description: "Тестовый режим",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	switch command.Command {
	case "OpenCheck":
		// Открытие чека - инициализация новой структуры документа
		currentDocument = consttypes.TDocument{
			IsTest:   command.Params.IsTest,
			IsReturn: command.Params.IsReturn,
			Cashier:  command.Params.Cashier,
			Items:    make([]consttypes.TItem, 0),
		}

		response.Success = true
		response.Message = "Чек открыт"

	case "Registration":
		// Добавление позиции в документ
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Добавляем позицию в документ: %v", command.Params)
		}
		item := consttypes.TItem{
			Name:     command.Params.Name,
			Price:    command.Params.Price,
			Quantity: command.Params.Quantity,
			Mark:     command.Params.Mark,
		}

		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Добавляем позицию в документ: %v", item)
		}

		currentDocument.Items = append(currentDocument.Items, item)

		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Документ после добавления позиции: %v", currentDocument)
		}

		response.Success = true
		response.Message = "Позиция добавлена"

	case "CloseCheck":
		// Добавляем суммы оплаты
		currentDocument.Cash = command.Params.Cash
		currentDocument.Ecash = command.Params.Ecash

		sessionKey := ""
		sessionKey, _, errOfCommand = merc.CheckStatsuConnectionKKT(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, "", config.UserMerc, config.PasswUserMerc)
		if errOfCommand != nil {
			response.Message = "Ошибка получения статуса чека: " + errOfCommand.Error()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		// Печать чека
		answer, errOfCommand = merc.PrintCheck(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, currentDocument, sessionKey, mercSNODefault, false, config.UserMerc, config.PasswUserMerc, false)
		if errOfCommand != nil {
			response.Message = "Ошибка печати чека: " + errOfCommand.Error()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		descrErr, errCloseSeesion := merc.Closesession(config.KktIP, config.KktPort, &sessionKey)
		if errCloseSeesion != nil {
			response.Message = "Ошибка закрытия сессии: " + descrErr
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Очищаем документ после печати
		currentDocument = consttypes.TDocument{}

	case "OpenShift":
		// Открытие смены
		answer, errOfCommand = merc.OpenCloseShift(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, config.UserMerc, config.PasswUserMerc, config.ComPort, "", true, command.Params.Cashier)
		if errOfCommand != nil {
			response.Message = "Ошибка открытия смены: " + errOfCommand.Error()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

	case "CloseShift":
		// Закрытие смены
		//sessionkey, descrError, err := merc.CheckStatsuConnectionKKT(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, "", config.UserMerc, config.PasswUserMerc)
		//if err != nil {
		//	response.Message = "Ошибка подключения к ККТ: " + descrError
		//	w.Header().Set("Content-Type", "application/json")
		//	json.NewEncoder(w).Encode(response)
		//	return
		//}
		answer, errOfCommand = merc.OpenCloseShift(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, config.UserMerc, config.PasswUserMerc, config.ComPort, "", false, command.Params.Cashier)
		if errOfCommand != nil {
			response.Message = "Ошибка закрытия смены: " + errOfCommand.Error()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

	case "PrintReport":
		// X-отчет
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Выполняем команду PrintReport")
		}
		answer, errOfCommand = merc.PrintReport(config.KktEmulation, config.KktIP, config.KktPort, config.ComPort, config.UserMerc, config.PasswUserMerc, command.Params.ReportCode, "")
		if errOfCommand != nil {
			response.Message = "Ошибка печати X-отчета: " + errOfCommand.Error()
			if consttypes.Logger != nil {
				consttypes.Logger.Printf("Ошибка печати X-отчета: %v", errOfCommand)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		if consttypes.Logger != nil {
			consttypes.Logger.Printf("Выполнена команда PrintReport")
			consttypes.Logger.Printf("Ответ: %s", answer)
		}
		err := json.Unmarshal([]byte(answer), &response.Answer)
		if err != nil {
			response.Message = "Ошибка разбора JSON ответа: " + err.Error()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		if response.Answer.Result != 0 {
			response.Message = "Ошибка печати X-отчета: " + response.Answer.Description
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

	default:
		response.Message = "Неизвестная команда: " + command.Command
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Обработка ответа от ККТ
	if answer != "" {
		var answerJson consttypes.TAnswerMercur
		err := json.Unmarshal([]byte(answer), &answerJson)
		if err != nil {
			response.Message = "Ошибка разбора JSON ответа: " + err.Error()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		response.Success = true
		response.Message = "Команда выполнена успешно"
		response.FiscNumb = fmt.Sprintf("%d", answerJson.FiscalDocNum)
		response.FiscSign = answerJson.FiscalSign
		response.ShiftNum = answerJson.ShiftNum
		response.Answer = answerJson
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	if consttypes.Logger != nil {
		consttypes.Logger.Printf("Обработка настроек: %s", r.Method)
	}

	switch r.Method {
	case http.MethodGet:
		if consttypes.Logger != nil {
			consttypes.Logger.Println("Получение текущих настроек")
		}
		mu.RLock()
		json.NewEncoder(w).Encode(config)
		mu.RUnlock()

	case http.MethodPost:
		if consttypes.Logger != nil {
			consttypes.Logger.Println("Обновление настроек")
		}
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			if consttypes.Logger != nil {
				consttypes.Logger.Printf("Ошибка декодирования настроек: %v", err)
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		config = newConfig
		mu.Unlock()

		path, err := saveConfig()
		w.Write([]byte("Файл настроек записан по пути: " + path))
		if err != nil {
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
	// Инициализируем логгер в начале main
	//logger.Println("Запуск программы...")

	//if runtime.GOOS == "windows" {
	//	// Установить совместимость с Windows 7
	//	os.Setenv("GODEBUG", "netdns=go")
	//}

	fmt.Println("Проверяем права администратора...")
	//if !service.Interactive() {
	//	isAdmin, err := isUserAdmin()
	//	if err != nil || !isAdmin {
	//		log.Fatal("Программа должна быть запущена с правами администратора")
	//		return
	//	}
	//}

	fmt.Println("Загружаем конфигурацию...")
	if err := loadConfig(); err != nil {
		log.Printf("Ошибка загрузки конфига: %v", err)
	}

	//if err := initLogger(); err != nil {
	//	//log.Fatal("Ошибка инициализации логгера:", err)
	//	fmt.Println("Ошибка инициализации логгера:", err)
	//}

	fmt.Println("Создаем службу...")
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal("Ошибка создания службы: ", err)
	}

	fmt.Println("Проверяем аргументы командной строки...")
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			fmt.Println("Начинаем установку службы...")
			err = s.Install()
			if err != nil {
				errMsg := fmt.Sprintf("Ошибка установки службы: %v", err)
				fmt.Println(errMsg)
				log.Fatal(err)
			}
			fmt.Println("Служба успешно установлена")
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
			err = s.Start()
			if err != nil {
				errMsg := fmt.Sprintf("Ошибка запуска службы: %v", err)
				fmt.Println(errMsg)
				log.Fatal(err)
			}
			fmt.Println("Служба успешно запущена")
			return
		case "stop":
			fmt.Println("Начинаем остановку службы...")
			err = s.Stop()
			if err != nil {
				errMsg := fmt.Sprintf("Ошибка остановки службы: %v", err)
				fmt.Println(errMsg)
				log.Fatal(err)
			}
			fmt.Println("Служба успешно остановлена")
			return
		case "run":
			runAsApplication()
			return
		}
	}

	err = s.Run()
	fmt.Println("Служба запущена", err)
}

//func isUserAdmin() (bool, error) {
//	fmt.Println("Проверяем права администратора...")
//	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
//	if err != nil {
//		return false, err
//	}
//	return true, nil
//}
