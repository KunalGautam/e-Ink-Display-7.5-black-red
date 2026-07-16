package db

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestInitDBAndDefaultSettings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Verify defaults
	val, err := database.GetSetting("port")
	if err != nil {
		t.Errorf("Error getting port: %v", err)
	}
	if val != "8080" {
		t.Errorf("Expected port to be 8080, got %s", val)
	}

	ff, err := database.GetSetting("font_family")
	if err != nil {
		t.Errorf("Error getting font_family: %v", err)
	}
	if ff != "Noto Sans Devanagari" {
		t.Errorf("Expected font_family to be Noto Sans Devanagari, got %s", ff)
	}

	user, hash, err := database.GetAuthCredentials()
	if err != nil {
		t.Fatalf("Failed to get credentials: %v", err)
	}
	if user != "admin" {
		t.Errorf("Expected username admin, got %s", user)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin")) != nil {
		t.Error("Hashed password does not match default 'admin'")
	}
}

func TestSettingsGetAndSave(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	err = database.SaveSetting("custom_key", "custom_val")
	if err != nil {
		t.Fatalf("Failed to save setting: %v", err)
	}

	val, err := database.GetSetting("custom_key")
	if err != nil {
		t.Fatalf("Failed to get setting: %v", err)
	}
	if val != "custom_val" {
		t.Errorf("Expected custom_val, got %s", val)
	}
}

func TestAuthCredentialsUpdates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	err = database.SaveAuthCredentials("new_admin", "securePass123")
	if err != nil {
		t.Fatalf("Failed to update credentials: %v", err)
	}

	user, hash, err := database.GetAuthCredentials()
	if err != nil {
		t.Fatalf("Failed to get credentials: %v", err)
	}
	if user != "new_admin" {
		t.Errorf("Expected username new_admin, got %s", user)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("securePass123")) != nil {
		t.Error("Password verification failed for updated password")
	}
}

func TestNotesCachingLimits(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Save 12 notes sequentially
	var notesList []string
	for i := 1; i <= 12; i++ {
		notesList = append(notesList, "Note "+strconv.Itoa(i))
	}

	err = database.SaveNotes(notesList)
	if err != nil {
		t.Fatalf("Failed to save notes list: %v", err)
	}

	cached, err := database.GetCachedNotes()
	if err != nil {
		t.Fatalf("Failed to retrieve notes: %v", err)
	}

	if len(cached) != 10 {
		t.Errorf("Expected exactly 10 notes to be cached, got %d", len(cached))
	}

	// Verify Note 1 and Note 2 were deleted (oldest)
	for _, note := range cached {
		if note == "Note 1" || note == "Note 2" {
			t.Errorf("Found note %s that should have been pruned", note)
		}
	}
}

func TestEmailsCachingLimits(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sqlite-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Save 12 emails
	var emailList []EmailRecord
	for i := 1; i <= 12; i++ {
		emailList = append(emailList, EmailRecord{
			Sender:  "Sender " + strconv.Itoa(i),
			Subject: "Subject " + strconv.Itoa(i),
		})
	}

	err = database.SaveEmails(emailList)
	if err != nil {
		t.Fatalf("Failed to save emails list: %v", err)
	}

	cached, err := database.GetCachedEmails()
	if err != nil {
		t.Fatalf("Failed to retrieve emails: %v", err)
	}

	if len(cached) != 10 {
		t.Errorf("Expected exactly 10 emails to be cached, got %d", len(cached))
	}

	// Verify oldest emails (1 and 2) were pruned
	for _, email := range cached {
		if email.Sender == "Sender 1" || email.Sender == "Sender 2" {
			t.Errorf("Found email sender %s that should have been pruned", email.Sender)
		}
	}
}
