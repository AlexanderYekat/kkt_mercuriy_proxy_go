package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/kardianos/service"
)

var logger service.Logger

type program struct{}

func (p *program) Start(s service.Service) error {
	// Используем отдельную goroutine для запуска сервера
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	return nil
}

func (p *program) run() {
	// Настраиваем обработчик
	http.HandleFunc("/document", handleRequest)
	logger.Info("Сервер запущен на порту 2579")
	if err := http.ListenAndServe(":2579", nil); err != nil {
		logger.Error(err)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" { // В Go 1.10 лучше использовать строковые константы
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Создаем новый запрос
	proxyReq, err := http.NewRequest("POST", "http://localhost:2578/document", r.Body)
	if err != nil {
		http.Error(w, "Ошибка при создании запроса", http.StatusInternalServerError)
		return
	}

	// Копируем заголовки
	copyHeader(proxyReq.Header, r.Header)
	proxyReq.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Ошибка при отправке запроса", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Копируем заголовки ответа
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	// Копируем тело ответа
	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.Error(err)
	}
}

// Вспомогательная функция для копирования заголовков
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func main() {
	svcConfig := &service.Config{
		Name:        "ProxyService",
		DisplayName: "Proxy Service",
		Description: "Прокси-сервис для перенаправления запросов",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Обработка аргументов командной строки
	if len(os.Args) > 1 {
		err = handleCommand(s, os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	// Запускаем службу
	if err = s.Run(); err != nil {
		logger.Error(err)
	}
}

// Вспомогательная функция для обработки команд
func handleCommand(s service.Service, cmd string) error {
	switch cmd {
	case "install":
		err := s.Install()
		if err != nil {
			return fmt.Errorf("Не удалось установить службу: %v", err)
		}
		fmt.Println("Служба успешно установлена")
	case "uninstall":
		err := s.Uninstall()
		if err != nil {
			return fmt.Errorf("Не удалось удалить службу: %v", err)
		}
		fmt.Println("Служба успешно удалена")
	case "start":
		err := s.Start()
		if err != nil {
			return fmt.Errorf("Не удалось запустить службу: %v", err)
		}
		fmt.Println("Служба запущена")
	case "stop":
		err := s.Stop()
		if err != nil {
			return fmt.Errorf("Не удалось остановить службу: %v", err)
		}
		fmt.Println("Служба остановлена")
	default:
		return fmt.Errorf("Неизвестная команда: %s", cmd)
	}
	return nil
}
