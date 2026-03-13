package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/resend/resend-go/v3"
)

func main() {
	const unset = "\x00"
	var prefix, domain, inboxPath, since string
	var noClear, noCreate bool

	flag.StringVar(&prefix, "prefix", unset, "filter recipients by address prefix")
	flag.StringVar(&domain, "domain", "", "filter recipients by domain")
	flag.StringVar(&inboxPath, "path", "./inbox", "inbox directory path")
	flag.StringVar(&since, "since", "0s", "look back duration (e.g. 10000h)")
	flag.BoolVar(&noClear, "no-clear", false, "do not clean out inbox on startup")
	flag.BoolVar(&noCreate, "no-create", false, "fail if inbox directory does not exist")
	flag.Parse()

	if prefix == unset {
		prefix = defaultPrefix()
	}

	lookback, err := time.ParseDuration(since)
	if err != nil {
		log.Fatalf("invalid -since value: %v", err)
	}

	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Fatal("RESEND_API_KEY environment variable is required")
	}

	if err := setupInbox(inboxPath, noClear, noCreate); err != nil {
		log.Fatalf("inbox setup: %v", err)
	}

	writeFilterFile(inboxPath, prefix, domain)

	client := resend.NewClient(apiKey)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	lastChecked := time.Now().UTC().Add(-lookback)

	log.Printf("listening for emails (prefix=%q, domain=%q, inbox=%q)", prefix, domain, inboxPath)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down")
			return
		case <-ticker.C:
			lastChecked = poll(ctx, client, prefix, domain, inboxPath, lastChecked)
		}
	}
}

func setupInbox(path string, noClear, noCreate bool) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		if noCreate {
			return fmt.Errorf("inbox directory %q does not exist", path)
		}
		return os.MkdirAll(path, 0755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}
	if !noClear {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			os.Remove(filepath.Join(path, e.Name()))
		}
	}
	return nil
}

func defaultPrefix() string {
	var parts []string

	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		name = filepath.Base(name)
		name = strings.TrimSuffix(name, ".git")
		if name != "" {
			parts = append(parts, name)
		}
	}

	if user := os.Getenv("USER"); user != "" {
		parts = append(parts, user)
	}

	return strings.Join(parts, "-")
}

func writeFilterFile(inboxPath, prefix, domain string) {
	var pattern string
	if prefix != "" {
		pattern = prefix + "*"
	} else {
		pattern = "*"
	}
	if domain != "" {
		pattern += "@" + domain
	} else {
		pattern += "@*"
	}

	msg := fmt.Sprintf("resend-listener is listening for emails matching %s\n", pattern)
	os.WriteFile(filepath.Join(inboxPath, "filter.txt"), []byte(msg), 0644)
}

func matchesFilter(recipients []string, prefix, domain string) bool {
	for _, addr := range recipients {
		addr = strings.ToLower(addr)
		parts := strings.SplitN(addr, "@", 2)
		if len(parts) != 2 {
			continue
		}
		if prefix != "" && !strings.HasPrefix(parts[0], strings.ToLower(prefix)) {
			continue
		}
		if domain != "" && parts[1] != strings.ToLower(domain) {
			continue
		}
		return true
	}
	return false
}

func poll(ctx context.Context, client *resend.Client, prefix, domain, inboxPath string, after time.Time) time.Time {
	emails, err := client.Emails.Receiving.ListWithContext(ctx)
	if err != nil {
		log.Printf("error listing emails: %v", err)
		return after
	}

	latest := after
	for _, email := range emails.Data {
		created, err := time.Parse(time.RFC3339, email.CreatedAt)
		if err != nil {
			continue
		}
		if !created.After(after) {
			continue
		}

		if (prefix != "" || domain != "") && !matchesFilter(email.To, prefix, domain) {
			if created.After(latest) {
				latest = created
			}
			continue
		}

		full, err := client.Emails.Receiving.GetWithContext(ctx, email.Id)
		if err != nil {
			log.Printf("error getting email %s: %v", email.Id, err)
			continue
		}

		var content []byte
		if full.Raw.DownloadUrl != "" {
			content, err = downloadRaw(ctx, full.Raw.DownloadUrl)
			if err != nil {
				log.Printf("error downloading raw email %s: %v", email.Id, err)
				continue
			}
		} else {
			content = buildRFC5322(full)
		}

		filename := generateFilename(inboxPath, full.CreatedAt)
		if err := os.WriteFile(filepath.Join(inboxPath, filename), content, 0644); err != nil {
			log.Printf("error writing email %s: %v", email.Id, err)
			continue
		}

		if created.After(latest) {
			latest = created
		}
		log.Printf("saved %s -> %s", email.Id, filename)
	}

	return latest
}

func downloadRaw(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func buildRFC5322(email *resend.ReceivedEmail) []byte {
	var b strings.Builder

	if t, err := time.Parse(time.RFC3339, email.CreatedAt); err == nil {
		b.WriteString(fmt.Sprintf("Date: %s\r\n", t.Format(time.RFC1123Z)))
	}
	b.WriteString(fmt.Sprintf("From: %s\r\n", email.From))
	b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(email.To, ", ")))
	if len(email.Cc) > 0 {
		b.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(email.Cc, ", ")))
	}
	if email.Subject != "" {
		b.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))
	}
	if email.MessageId != "" {
		b.WriteString(fmt.Sprintf("Message-ID: %s\r\n", email.MessageId))
	}
	for k, v := range email.Headers {
		b.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	if email.Text != "" {
		b.WriteString(email.Text)
	} else {
		b.WriteString(email.Html)
	}

	return []byte(b.String())
}

func generateFilename(inboxPath, createdAt string) string {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		t = time.Now()
	}

	base := t.Format("20060102-150405")

	candidate := base + ".eml"
	if _, err := os.Stat(filepath.Join(inboxPath, candidate)); os.IsNotExist(err) {
		return candidate
	}

	for i := 1; ; i++ {
		candidate = fmt.Sprintf("%s%d.eml", base, i)
		if _, err := os.Stat(filepath.Join(inboxPath, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}
