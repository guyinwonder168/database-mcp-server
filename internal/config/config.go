// config.go
package config

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

var ErrDecryptionFailed = errors.New("decrypt password failed")

type Profile struct {
	ProfileName  string `yaml:"profile_name"`
	DBType       string `yaml:"db_type"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	DatabaseName string `yaml:"database_name"`
	Readonly     bool   `yaml:"readonly"`
	SSLMode      string `yaml:"sslmode,omitempty"` // Postgres SSL mode (disable, require, verify-ca, verify-full)
}

type NLPConfig struct {
	Enabled               *bool    `yaml:"enabled"`
	ContextTimeout        string   `yaml:"context_timeout"`
	MaxConversationLength int      `yaml:"max_conversation_length"`
	BusinessDomains       []string `yaml:"business_domains,omitempty"`
}

type Config struct {
	Profiles    []Profile `yaml:"profiles"`
	MaxPoolSize int       `yaml:"max_pool_size"`
	AESKey      string    `yaml:"aes_key"`
	NLP         NLPConfig `yaml:"nlp,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	// Decrypt passwords (fail fast on any decryption error for security clarity)
	key := cfg.AESKey
	if key != "" && len(key) != 32 {
		return nil, fmt.Errorf("invalid AES key length (%d) - must be 32: %w", len(key), ErrDecryptionFailed)
	}
	for i := range cfg.Profiles {
		if key != "" && cfg.Profiles[i].Password != "" {
			plain, derr := decryptAESGCM(cfg.Profiles[i].Password, key)
			if derr != nil {
				return nil, fmt.Errorf("decrypt password for profile '%s': %w: %v", cfg.Profiles[i].ProfileName, ErrDecryptionFailed, derr)
			}
			cfg.Profiles[i].Password = plain
		}
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	key := cfg.AESKey
	// Encrypt passwords
	tmp := *cfg
	for i := range tmp.Profiles {
		if key != "" && tmp.Profiles[i].Password != "" {
			enc, err := encryptAESGCM(tmp.Profiles[i].Password, key)
			if err == nil {
				tmp.Profiles[i].Password = enc
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := yaml.NewEncoder(f)
	defer encoder.Close()
	return encoder.Encode(&tmp)
}

// PromptForProfiles interactively collects one or more profiles from the user via CLI, with validation.
func PromptForProfiles() ([]Profile, int, string) {
	var profiles []Profile
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("No config.yaml found. Let's create one or more database connection profiles.")
	for {
		var p Profile
		fmt.Print("Profile name: ")
		scanner.Scan()
		p.ProfileName = scanner.Text()
		if p.ProfileName == "" {
			fmt.Println("Profile name cannot be empty.")
			continue
		}

		for {
			fmt.Print("Database type (mysql/mariadb/postgres/sqlite): ")
			scanner.Scan()
			p.DBType = scanner.Text()
			switch p.DBType {
			case "mysql", "mariadb", "postgres", "sqlite":
				// valid
			default:
				fmt.Println("Invalid database type. Please enter one of: mysql, mariadb, postgres, sqlite.")
				continue
			}
			break
		}

		if p.DBType != "sqlite" {
			fmt.Print("Host: ")
			scanner.Scan()
			p.Host = scanner.Text()
			fmt.Print("Port: ")
			scanner.Scan()
			fmt.Sscanf(scanner.Text(), "%d", &p.Port)
			fmt.Print("Username: ")
			scanner.Scan()
			p.Username = scanner.Text()
			fmt.Print("Password: ")
			scanner.Scan()
			p.Password = scanner.Text()
			fmt.Print("Database name: ")
			scanner.Scan()
			p.DatabaseName = scanner.Text()
		} else {
			fmt.Print("SQLite file path: ")
			scanner.Scan()
			p.DatabaseName = scanner.Text()
		}

		for {
			fmt.Print("Readonly profile? (true/false): ")
			scanner.Scan()
			readonlyInput := scanner.Text()
			if readonlyInput == "true" {
				p.Readonly = true
				break
			} else if readonlyInput == "false" {
				p.Readonly = false
				break
			} else {
				fmt.Println("Please enter 'true' or 'false'.")
			}
		}

		profiles = append(profiles, p)

		fmt.Print("Add another profile? (y/n): ")
		scanner.Scan()
		if scanner.Text() != "y" {
			break
		}
	}

	// Prompt for max pool size
	var maxPoolSize int
	for {
		fmt.Print("Set maximum database pool size (recommended 5-50): ")
		scanner.Scan()
		_, err := fmt.Sscanf(scanner.Text(), "%d", &maxPoolSize)
		if err != nil || maxPoolSize < 1 {
			fmt.Println("Please enter a valid positive integer.")
			continue
		}
		break
	}

	// Prompt for AES key
	var aesKey string
	for {
		fmt.Print("Set AES key for password encryption (32 chars, leave blank for insecure): ")
		scanner.Scan()
		aesKey = scanner.Text()
		if aesKey == "" || len(aesKey) == 32 {
			break
		}
		fmt.Println("AES key must be exactly 32 characters (256 bits).")
	}

	return profiles, maxPoolSize, aesKey
}

// AES-GCM helpers for password encryption

func encryptAESGCM(plaintext, keyStr string) (string, error) {
	key := []byte(keyStr)
	if len(key) != 32 {
		return "", errors.New("AES key must be 32 bytes (256 bits)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptAESGCM(ciphertextB64, keyStr string) (string, error) {
	key := []byte(keyStr)
	if len(key) != 32 {
		return "", errors.New("AES key must be 32 bytes (256 bits)")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aesgcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
