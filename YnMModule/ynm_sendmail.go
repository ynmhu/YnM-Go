package YnMModule

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"crypto/tls"
	"io/ioutil"
	"net/smtp"
		"git.ynm.hu/markus/YnM-Go/YnMConfig"
)

type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

var SMTP SMTPConfig

// LoadSMTPConfig betölti az SMTP beállításokat a YAML fájlból
func LoadSMTPConfig(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	config := struct {
		SMTP SMTPConfig `yaml:"smtp"`
	}{}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	SMTP = config.SMTP
	return nil
}

// SendEmailInsecure elküld egy e-mailt, és elfogadja a self-signed tanúsítványt
func SendEmail(cfg *YnMConfig.Config, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTP.Host, cfg.SMTP.Port)
	auth := smtp.PlainAuth("", cfg.SMTP.User, cfg.SMTP.Password, cfg.SMTP.Host)

	// TLS konfiguráció self-signed tanúsítványhoz
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,       // figyelem: nem biztonságos éles környezetben
		ServerName:         cfg.SMTP.Host,
	}

	// TLS kapcsolat létrehozása
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		fmt.Printf("[DEBUG SendEmail] TLS connection failed: %v\n", err)
		return err
	}

	// SMTP kliens létrehozása
	client, err := smtp.NewClient(conn, cfg.SMTP.Host)
	if err != nil {
		fmt.Printf("[DEBUG SendEmail] SMTP client creation failed: %v\n", err)
		return err
	}
	defer client.Quit()

	// Authentikáció
	if err := client.Auth(auth); err != nil {
		fmt.Printf("[DEBUG SendEmail] SMTP auth failed: %v\n", err)
		return err
	}

	// Feladó és címzett beállítása
	if err := client.Mail(cfg.SMTP.User); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	// Üzenet küldése
	w, err := client.Data()
	if err != nil {
		return err
	}
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		cfg.SMTP.User, to, subject, body))
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	w.Close()

	fmt.Printf("[DEBUG SendEmail] Email sent to %s successfully\n", to)
	return nil
}