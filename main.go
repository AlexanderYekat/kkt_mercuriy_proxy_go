package main

import (
	"io"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/document", handleRequest)
	log.Println("Сервер запущен на порту 2579")
	log.Fatal(http.ListenAndServe(":2579", nil))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Создаем новый запрос к целевому серверу
	targetURL := "http://localhost:2578/document"
	proxyReq, err := http.NewRequest(http.MethodPost, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Ошибка при создании запроса", http.StatusInternalServerError)
		return
	}

	// Копируем заголовки из оригинального запроса
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Добавляем заголовок Content-Type: application/json
	proxyReq.Header.Set("Content-Type", "application/json")

	// Отправляем запрос к целевому серверу
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Ошибка при отправке запроса", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Копируем статус и заголовки ответа
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Копируем тело ответа
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Ошибка при копировании ответа: %v", err)
	}
}
