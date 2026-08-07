package rename

import (
	"errors"
	"path"
	"regexp"
	"strconv"
	"strings"

	"jproxy-go/internal/format"
)

var (
	ErrInvalidPath  = errors.New("invalid rename path")
	videoOrSubtitle = regexp.MustCompile(`(?i)(\.(mp4|avi|wmv|flv|mov|mkv|webm|mpg|mpeg|3gp|iso|ts|([-_a-z]{2,5}\.|)ass|([-_a-z]{2,5}\.|)srt|([-_a-z]{2,5}\.|)ssa|([-_a-z]{2,5}\.|)idx|([-_a-z]{2,5}\.|)sub))$`)
	subtitle        = regexp.MustCompile(`(?i)\.(ass|srt|ssa|idx|sub)$`)
	episode         = regexp.MustCompile(`(S\d+|\b|\s)E\d+`)
)

type FileRename struct {
	OldPath string
	NewPath string
}

func PlanRadarrFiles(sourceTitle string, files []string) ([]FileRename, error) {
	return planFiles(sourceTitle, files, func(name string, subtitleNumber int) (string, int) {
		return radarrName(sourceTitle, name, subtitleNumber)
	})
}

func PlanSonarrFiles(sourceTitle string, files []string, pattern string, rules []format.Rule) ([]FileRename, error) {
	return planFiles(sourceTitle, files, func(name string, subtitleNumber int) (string, int) {
		return sonarrName(sourceTitle, name, pattern, rules, subtitleNumber)
	})
}

func planFiles(sourceTitle string, files []string, makeName func(string, int) (string, int)) ([]FileRename, error) {
	if !validName(sourceTitle) {
		return nil, ErrInvalidPath
	}
	for _, oldPath := range files {
		if !validPath(oldPath) {
			return nil, ErrInvalidPath
		}
	}
	result := make([]FileRename, 0, len(files))
	subtitleNumber := 1
	for _, oldPath := range files {
		if path.Dir(oldPath) == sourceTitle {
			return nil, nil
		}
		name, nextSubtitle := makeName(path.Base(oldPath), subtitleNumber)
		segments := strings.Split(oldPath, "/")
		var newPath string
		if len(segments) == 1 {
			newPath = sourceTitle + "/" + name
		} else {
			newPath = sourceTitle + "/" + strings.Join(segments[1:len(segments)-1], "/")
			if len(segments) > 2 {
				newPath += "/"
			}
			newPath += name
		}
		if !validPath(newPath) {
			return nil, ErrInvalidPath
		}
		result = append(result, FileRename{OldPath: oldPath, NewPath: newPath})
		subtitleNumber = nextSubtitle
	}
	return result, nil
}

func radarrName(title, oldName string, subtitleNumber int) (string, int) {
	match := videoOrSubtitle.FindStringSubmatch(oldName)
	if match == nil {
		return oldName, subtitleNumber
	}
	extension := match[1]
	if subtitle.MatchString(extension) {
		extension = "." + strconv.Itoa(subtitleNumber) + extension
		subtitleNumber++
	}
	return withSuffix(title, title) + extension, subtitleNumber
}

func sonarrName(title, oldName, pattern string, rules []format.Rule, subtitleNumber int) (string, int) {
	match := videoOrSubtitle.FindStringSubmatch(oldName)
	if match == nil {
		return oldName, subtitleNumber
	}
	base, extension := strings.TrimSuffix(oldName, match[1]), match[1]
	formatted := format.FormatTokens(base, pattern, rules)
	if strings.TrimSpace(formatted) == "" || !episode.MatchString(formatted) {
		return oldName, subtitleNumber
	}
	if subtitle.MatchString(extension) {
		extension = "." + strconv.Itoa(subtitleNumber) + extension
		subtitleNumber++
	}
	return withSuffix(formatted, title) + extension, subtitleNumber
}

func withSuffix(name, title string) string {
	if index := strings.Index(title, " ["); index >= 0 {
		return name + title[index:]
	}
	return name
}

func validPath(value string) bool {
	if value == "" || hasControl(value) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "." || component == ".." || component == "" {
			return false
		}
	}
	return true
}

func validName(value string) bool {
	return validPath(value) && !strings.Contains(value, "/")
}
