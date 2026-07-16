package renderer

import (
	"image/png"
	"os"
	"testing"

	"epaper-display/go-server/internal/config"
	"epaper-display/go-server/internal/mqtt"
)

func TestRenderer(t *testing.T) {
	// 1. Download/Verify fonts are available for test
	regPath, boldPath, err := EnsureFontsDownloaded("Noto Sans Devanagari")
	if err != nil {
		t.Fatalf("Failed to ensure fonts are downloaded: %v", err)
	}

	// 2. Mock configuration
	cfg := &config.Config{
		Width:  800,
		Height: 480,
	}

	// 3. Initialize renderer
	r := NewRenderer(cfg, regPath, boldPath)

	// 4. Sample data (incorporating Devanagari text)
	notes := []string{
		"सब्जियां और फल बाजार से खरीदना न भूलें (Buy groceries and milk)",
		"Review the implementation plan for epaper display system and sign off",
		"अमित के साथ शाम 6:00 बजे मीटिंग (Meet Amit at 6:00 PM)",
		"Water the backyard plants at 6:00 PM today",
	}

	emails := []mqtt.Email{
		{Sender: "अमित शर्मा <amit@example.com>", Subject: "मीटिंग का समय बदलाव (Meeting reschedule request)"},
		{Sender: "Github Notifications", Subject: "[PR #402] Merged: Add layout engine functionality"},
		{Sender: "Tech Newsletter", Subject: "Top 10 Raspberry Pi hardware projects of 2026"},
		{Sender: "Family Group", Subject: "Sunday dinner plans and updates"},
	}

	calendarEvents := []mqtt.CalendarEvent{
		{Title: "सब्जी मंडी जाना (All Day Groceries)", Time: "Today, All Day"},
		{Title: "अमित के साथ बैठक (Meeting with Amit)", Time: "Tom, 09:00 - 10:00"},
		{Title: "Review layout design", Time: "Mon 13, 14:00 - 15:00"},
	}

	// 5. Render
	// Run renderer.py script in go-server folder. Since test is executed in go-server/internal/renderer,
	// renderer.py is located at ../../renderer.py.
	// But our main.go calls it in go-server.
	// For testing, let's make sure the command runs in go-server folder.
	// Wait, we can change the current directory in tests or just set cmd.Dir.
	// Let's modify renderer.go or execute tests with Cwd.
	// Wait! In renderer.go:
	// cmd := exec.Command("python3", "./renderer.py")
	// If the test runs in go-server/internal/renderer, "./renderer.py" won't find it!
	// So cmd should run in the server root. How do we make cmd find it?
	// We can check if "./renderer.py" exists, and if not, try "../renderer.py" or "../../renderer.py".
	// Or even simpler: we can set `cmd.Dir` in renderer.go if we can locate the root, or let config configure it.
	// Even simpler: in `renderer.go`, we can check:
	// scriptPath := "./renderer.py"
	// if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
	//     // try parent directories
	//     if _, err := os.Stat("../renderer.py"); err == nil {
	//         scriptPath = "../renderer.py"
	//     } else if _, err := os.Stat("../../renderer.py"); err == nil {
	//         scriptPath = "../../renderer.py"
	//     }
	// }
	// This is extremely robust! It guarantees the server can find and run renderer.py whether it's launched
	// from the project root, cmd/server/, or from inside unit tests!
	// Let's implement this path discovery inside renderer.go first or edit it.
	// Let's verify: Yes, that is a brilliant detail.

	img, err := r.Render(notes, emails, calendarEvents, "2026-07-12 16:11:51")
	if err != nil {
		t.Fatalf("Renderer.Render returned unexpected error: %v", err)
	}

	if img == nil {
		t.Fatal("Renderer.Render returned nil image")
	}

	// Verify dimensions
	bounds := img.Bounds()
	if bounds.Dx() != 800 || bounds.Dy() != 480 {
		t.Errorf("Unexpected image dimensions: got %dx%d, expected 800x480", bounds.Dx(), bounds.Dy())
	}

	// 6. Save image to disk as local verification artifact
	outputPath := "test_render_output.png"
	f, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("Failed to encode test PNG: %v", err)
	}
	t.Logf("Saved test render preview to: %s", outputPath)

	// 7. Verify byte packing
	blackBuf, redBuf := PackBuffers(img)

	expectedBufSize := (800 * 480) / 8 // 48000 bytes
	if len(blackBuf) != expectedBufSize {
		t.Errorf("Unexpected black buffer size: got %d, expected %d", len(blackBuf), expectedBufSize)
	}
	if len(redBuf) != expectedBufSize {
		t.Errorf("Unexpected red buffer size: got %d, expected %d", len(redBuf), expectedBufSize)
	}
}
