package slackdump

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitfield/script"
	"github.com/google/uuid"
	"github.com/rusq/slack"
	"github.com/rusq/slackdump/v3/types"
)

func Dump(ctx context.Context, url string) ([]string, error) {
	conversation, err := dumpThread(url)
	if err != nil {
		return nil, fmt.Errorf("failed to dump thread: %w", err)
	}

	users, err := GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	userMap := users.IndexByID()

	messages := []string{}

	for _, message := range conversation.Messages {
		// Decode escaped Unicode characters (e.g. \u003c becomes <)
		decoded := strings.ReplaceAll(message.Text, `\u003c`, "<")
		decoded = strings.ReplaceAll(decoded, `\u003e`, ">")
		decoded = strings.ReplaceAll(decoded, `\u0026gt;`, ">")
		decoded = strings.ReplaceAll(decoded, `\u0026amp;`, "&")

		// Regular expression to match Slack user mentions: <@USERID>
		re := regexp.MustCompile(`<@([A-Z0-9]+)>`)
		converted := re.ReplaceAllStringFunc(decoded, func(m string) string {
			// Extract just the USERID
			matches := re.FindStringSubmatch(m)
			if len(matches) > 1 {
				if user, ok := userMap[matches[1]]; ok {
					return user.Name
				}
			}
			return m // fallback to original if not found
		})

		user := userMap.Sender(&message.Message)
		result := user + ": " + converted

		messages = append(messages, result)
	}

	return messages, nil
}

func dumpThread(url string) (*types.Conversation, error) {
	filePath := filepath.Join(os.TempDir(), uuid.New().String()+".zip")
	defer os.Remove(filePath)

	cmd := `slackdump dump -o ` + filePath + ` -v ` + url
	_, err := script.Exec(cmd).String()
	if err != nil {
		return nil, fmt.Errorf("failed execute slackdump dump: %w, cmd: %s", err, cmd)
	}

	result, err := readDumpJson(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dump json: %w, cmd: %s", err, cmd)
	}

	var conversation types.Conversation
	if err := json.Unmarshal(result, &conversation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dump json: %w, cmd: %s", err, cmd)
	}

	return &conversation, nil
}

func readDumpJson(filePath string) ([]byte, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w, filePath: %s", err, filePath)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".json") {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open json file: %w, filePath: %s", err, f.Name)
			}
			defer rc.Close()

			return io.ReadAll(rc)
		}
	}

	return nil, errors.New("no json file found")
}

func GetUsers(ctx context.Context) (types.Users, error) {
	result, err := script.Exec(`slackdump list users -no-json`).String()
	if err != nil {
		return nil, fmt.Errorf("failed to execute slackdump list users: %w", err)
	}

	return parseUsers(ctx, result)
}

func parseUsers(_ context.Context, result string) (types.Users, error) {
	users := []slack.User{}

	lines := strings.Split(result, "\n")

	// skip header
	lines = lines[2:]

	for _, line := range lines {
		user := slack.User{}
		cols := strings.Fields(line)
		if len(cols) < 2 {
			log.Printf("skipping line: %s", line)
			continue
		}

		user.Name = cols[0]
		user.ID = cols[1]

		cols = cols[2:]

		for _, col := range cols {
			text := strings.TrimSpace(col)
			if strings.EqualFold(text, "bot") {
				user.IsBot = true
			}

			if strings.EqualFold(text, "deleted") {
				user.Deleted = true
			}

			if strings.EqualFold(text, "restricted") {
				user.IsRestricted = true
			}

			if strings.Contains(text, "@") {
				user.Profile.Email = text
			}
		}

		users = append(users, user)
	}

	return types.Users(users), nil
}
