package i18n

import (
	"encoding/json"
	"io/fs"
	"log"
	"strings"
)

type Localizer struct {
	translations map[string]map[string]string
}

func New(fsys fs.FS) *Localizer {
	translations := make(map[string]map[string]string)

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
			lang := strings.TrimSuffix(d.Name(), ".json")
			file, err := fsys.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			var langMap map[string]string
			if err := json.NewDecoder(file).Decode(&langMap); err != nil {
				return err
			}
			translations[lang] = langMap
			log.Printf("Loaded language file: %s", path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to load language files: %v", err)
	}

	return &Localizer{translations: translations}
}

func (l *Localizer) Get(lang, key string) string {
	if langMap, ok := l.translations[lang]; ok {
		if value, ok := langMap[key]; ok {
			return value
		}
	}

	if langMap, ok := l.translations["en"]; ok {
		if value, ok := langMap[key]; ok {
			return value
		}
	}
	return key
}