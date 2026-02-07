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
	"strings"

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
	// #nosec G304 -- config path is provided by trusted caller
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
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
	// #nosec G304 -- config path is provided by trusted caller
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	encoder := yaml.NewEncoder(f)
	defer encoder.Close() //nolint:errcheck // Standard pattern: error in deferred close is not critical
	return encoder.Encode(&tmp)
}

// PromptForProfiles interactively collects one or more profiles from the user via CLI, with validation.
func PromptForProfiles() ([]Profile, int, string) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("No config.yaml found. Let's create one or more database connection profiles.")
	return collectProfiles(scanner), promptMaxPoolSize(scanner), promptAESKey(scanner)
}

func collectProfiles(scanner *bufio.Scanner) []Profile {
	profiles := make([]Profile, 0)
	for {
		profile, ok := promptProfile(scanner)
		if !ok {
			continue
		}
		profiles = append(profiles, profile)
		if readInput(scanner, "Add another profile? (y/n): ") != "y" {
			return profiles
		}
	}
}

func promptProfile(scanner *bufio.Scanner) (Profile, bool) {
	profileName := strings.TrimSpace(readInput(scanner, "Profile name: "))
	if profileName == "" {
		fmt.Println("Profile name cannot be empty.")
		return Profile{}, false
	}

	profile := Profile{
		ProfileName: profileName,
		DBType:      promptDBType(scanner),
	}

	if profile.DBType == "sqlite" {
		profile.DatabaseName = readInput(scanner, "SQLite file path: ")
	} else {
		profile.Host = readInput(scanner, "Host: ")
		profile.Port = promptPort(scanner)
		profile.Username = readInput(scanner, "Username: ")
		profile.Password = readInput(scanner, "Password: ")
		profile.DatabaseName = readInput(scanner, "Database name: ")
	}

	profile.Readonly = promptReadonly(scanner)
	return profile, true
}

func promptDBType(scanner *bufio.Scanner) string {
	for {
		dbType := readInput(scanner, "Database type (mysql/mariadb/postgres/sqlite): ")
		switch dbType {
		case "mysql", "mariadb", "postgres", "sqlite":
			return dbType
		default:
			fmt.Println("Invalid database type. Please enter one of: mysql, mariadb, postgres, sqlite.")
		}
	}
}

func promptPort(scanner *bufio.Scanner) int {
	for {
		input := readInput(scanner, "Port: ")
		var port int
		if _, err := fmt.Sscanf(input, "%d", &port); err == nil {
			return port
		}
		fmt.Println("Invalid port. Please enter a number.")
	}
}

func promptReadonly(scanner *bufio.Scanner) bool {
	for {
		readonlyInput := readInput(scanner, "Readonly profile? (true/false): ")
		switch readonlyInput {
		case "true":
			return true
		case "false":
			return false
		default:
			fmt.Println("Please enter 'true' or 'false'.")
		}
	}
}

func promptMaxPoolSize(scanner *bufio.Scanner) int {
	for {
		input := readInput(scanner, "Set maximum database pool size (recommended 5-50): ")
		var maxPoolSize int
		if _, err := fmt.Sscanf(input, "%d", &maxPoolSize); err == nil && maxPoolSize > 0 {
			return maxPoolSize
		}
		fmt.Println("Please enter a valid positive integer.")
	}
}

func promptAESKey(scanner *bufio.Scanner) string {
	for {
		aesKey := readInput(scanner, "Set AES key for password encryption (32 chars, leave blank for insecure): ")
		if aesKey == "" || len(aesKey) == 32 {
			return aesKey
		}
		fmt.Println("AES key must be exactly 32 characters (256 bits).")
	}
}

func readInput(scanner *bufio.Scanner, prompt string) string {
	fmt.Print(prompt)
	scanner.Scan()
	return scanner.Text()
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
