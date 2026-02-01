package main

import (
	"log"
	"os"

	ftpserver "github.com/fclairamb/ftpserverlib"
)

func main() {
	nextcloudURL := os.Getenv("NEXTCLOUD_URL")
	if nextcloudURL == "" {
		log.Fatal("NEXTCLOUD_URL environment variable is required")
	}

	ftpPort := os.Getenv("FTP_PORT")
	if ftpPort == "" {
		ftpPort = "2121"
	}

	enableTLS := os.Getenv("FTP_TLS")
	debug := os.Getenv("DEBUG")
	insecureSkipVerify := os.Getenv("INSECURE_SKIP_VERIFY")

	driver := &NextCloudDriver{
		nextcloudURL:       nextcloudURL,
		enableTLS:          enableTLS == "true" || enableTLS == "1",
		debug:              debug == "true" || debug == "1",
		insecureSkipVerify: insecureSkipVerify == "true" || insecureSkipVerify == "1",
	}

	server := ftpserver.NewFtpServer(driver)

	log.Printf("FTP server listening on port %s", ftpPort)
	log.Printf("NextCloud URL: %s", nextcloudURL)
	if driver.enableTLS {
		log.Printf("FTPS (TLS) enabled")
	}
	if driver.debug {
		log.Printf("DEBUG mode enabled - logging all WebDAV requests/responses")
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("FTP server error: %v", err)
	}
}
