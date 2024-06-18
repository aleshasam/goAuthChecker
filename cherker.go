package main

import (
	"bytes"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file")
	}
}

func getServerNames(sshHost, sshUser, privateKeyFile string) ([]string, error) {
	configPath := os.Getenv("CONFIG_PATH")
	cmd := exec.Command("ssh", "-i", privateKeyFile, fmt.Sprintf("%s@%s", sshUser, sshHost), fmt.Sprintf("grep -hri -Poe 'server_name \\K[^; ]+' %s | sort -u", configPath))
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	serverNames := strings.Split(strings.TrimSpace(out.String()), "\n")
	return serverNames, nil
}

func getStatusCode(url string) (int, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "curl/7.68.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func isValidStatus(statusCode int) bool {
	return statusCode != 401 && statusCode > 0
}

func makeGetRequest(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "Go-Client/1.0")
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func sendMessages(messages []string) error {
	urlTemplate := os.Getenv("PUSH_TEMPlATE")

	message := strings.Join(messages, "")

	requestUrl := fmt.Sprintf(urlTemplate, url.QueryEscape(message))
	res, err := makeGetRequest(requestUrl)

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Result: ", res)
	return nil
}

func main() {
	sshHost := os.Getenv("SSH_HOST")
	sshUser := os.Getenv("SSH_USER")
	privateKeyFile := os.Getenv("PRIVATE_KEY_FILE")

	serverNames, err := getServerNames(sshHost, sshUser, privateKeyFile)
	if err != nil {
		fmt.Println("Error getting server names:", err)
		return
	}

	messages := []string{"-----------------------\n" + time.Now().Format("2006-01-02") + "\n-----------------------\n"}

	allCount := len(serverNames)
	for key, serverName := range serverNames {
		fmt.Printf("Checking %d/%d %s...\r", key+1, allCount, serverName)

		httpStatus, err := getStatusCode("http://" + serverName)
		if err == nil && isValidStatus(httpStatus) {
			messages = append(messages, fmt.Sprintf("http %s %d\n", serverName, httpStatus))
		}

		httpsStatus, err := getStatusCode("https://" + serverName)
		if err == nil && isValidStatus(httpsStatus) {
			messages = append(messages, fmt.Sprintf("https %s %d\n", serverName, httpsStatus))
		}
	}

	fmt.Println("\n\n\n==============\n\n\n")

	messagesChunked := chunkMessages(messages, 10)
	for _, chunk := range messagesChunked {
		if err := sendMessages(chunk); err != nil {
			fmt.Println("Error sending message:", err)
		}
	}
}

func chunkMessages(messages []string, chunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(messages); i += chunkSize {
		end := i + chunkSize
		if end > len(messages) {
			end = len(messages)
		}
		chunks = append(chunks, messages[i:end])
	}
	return chunks
}
