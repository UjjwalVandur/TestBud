package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	err := filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor") {
			return nil
		}
		
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(content)

		if !strings.Contains(text, "logrus") {
			return nil
		}

		// Replace imports
		text = strings.ReplaceAll(text, "\"github.com/sirupsen/logrus\"", "\"log/slog\"\n\t\"os\"")
		text = strings.ReplaceAll(text, "*logrus.Logger", "*slog.Logger")
		
		// In main/seed
		text = strings.ReplaceAll(text, "logrus.New()", "slog.New(slog.NewJSONHandler(os.Stdout, nil))")
		text = regexp.MustCompile(`(?m)^\s*logger\.SetFormatter.*$`).ReplaceAllString(text, "")
		text = regexp.MustCompile(`(?m)^\s*logger\.SetOutput.*$`).ReplaceAllString(text, "")
		
		// .WithError(err).Fatal(...) -> Error(...) then os.Exit(1)
		text = regexp.MustCompile(`logger\.WithError\(([^)]+)\)\.Fatal\(([^)]+)\)`).ReplaceAllString(text, "logger.Error($2, \"error\", $1); os.Exit(1)")
		
		// .WithError(err).Error(...) -> Error(...)
		text = regexp.MustCompile(`logger\.WithError\(([^)]+)\)\.Error\(([^)]+)\)`).ReplaceAllString(text, "logger.Error($2, \"error\", $1)")
		
		// .WithError(err).Warn(...) -> Warn(...)
		text = regexp.MustCompile(`logger\.WithError\(([^)]+)\)\.Warn\(([^)]+)\)`).ReplaceAllString(text, "logger.Warn($2, \"error\", $1)")
		
		// .WithError(err).WithField(...) -> WithField(...)
		text = regexp.MustCompile(`logger\.WithError\(([^)]+)\)\.WithField\(([^,]+),\s*([^)]+)\)\.Error\(([^)]+)\)`).ReplaceAllString(text, "logger.Error($4, \"error\", $1, $2, $3)")
		text = regexp.MustCompile(`logger\.WithError\(([^)]+)\)\.WithField\(([^,]+),\s*([^)]+)\)\.Warn\(([^)]+)\)`).ReplaceAllString(text, "logger.Warn($4, \"error\", $1, $2, $3)")
		
		// .WithField(k, v).Info(...)
		text = regexp.MustCompile(`logger\.WithField\(([^,]+),\s*([^)]+)\)\.Info\(([^)]+)\)`).ReplaceAllString(text, "logger.Info($3, $1, $2)")
		
		// .WithFields(logrus.Fields{...}).Info(...) / Warn(...)
		// This is multiline usually. We'll do it by replacing the block manually where it appears.
		
		os.WriteFile(path, []byte(text), info.Mode())
		fmt.Println("Processed", path)
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
}
