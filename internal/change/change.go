package change 

import (
	"bytes"
	"strings"
	"fmt"
	"regexp"

	"github.com/andygrunwald/go-gerrit"
)

const (
	FileDiffsLimit = 3 
)

type GerritChange struct {
	// GerritInfo     gerrit.ChangeInfo
	// GerritDiffInfo gerrit.DiffInfo
	Paths         []string
	Subject       string
	Type          string
	Project       string
	URL           string
	DiffSample    []byte
	DiffMap       map[string]string
	IsInteresting bool
}

func NewGerritChange(changeInfo *gerrit.ChangeInfo, diffsInfo []*gerrit.DiffInfo, endpointURL string) (GerritChange, error) {
	fpaths := make([]string, 0)
	diffMap := make(map[string]string, 0)
	for index, diffInfo := range diffsInfo {
		content := diffInfo.Content
		switch diffInfo.ChangeType {
		case "ADDED":
			fpaths = append(fpaths, diffInfo.MetaB.Name)
			if index > FileDiffsLimit {
				continue
			}
			var buf bytes.Buffer
			for _, data := range content {
				buf.WriteString(strings.Join(data.B, "\r\n"))
			}
			diffMap[diffInfo.MetaB.Name] = buf.String()
		case "MODIFIED":
			if index > FileDiffsLimit {
				continue
			}
			fpaths = append(fpaths, diffInfo.MetaA.Name)
			var buf bytes.Buffer
			for _, data := range content {
				buf.WriteString(strings.Join(data.B, "\r\n"))
			}
			diffMap[diffInfo.MetaB.Name] = buf.String()
		}
	}

	return GerritChange{Paths: fpaths, Type: "dummy",
		Subject: changeInfo.Subject, Project: changeInfo.Project,
		DiffMap: diffMap,
		URL:     extractChangeId(changeInfo.ChangeID, endpointURL)}, nil
}

func extractChangeId(rawChangeId string, endpointURL string) string {
	regexp, err := regexp.Compile(`[A-Za-z0-9]{32,}`)
	if err != nil {
		return rawChangeId
	}
	changeId := regexp.FindString(rawChangeId)
	if changeId != "" {
		return fmt.Sprintf("%s/q/%s", endpointURL, changeId)
	}
	return rawChangeId
}
