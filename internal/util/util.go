package util 

import (
	"github.com/andygrunwald/go-gerrit"
	"strings"
	"sort"
)

func FilterFiles(files map[string]gerrit.FileInfo) []string {
	filtered := make([]string, 0)
	for f := range files {
		if !strings.HasSuffix(f, ".gn") && !strings.HasPrefix(f, "testing") &&
			!strings.HasSuffix(f, ".xml") && !strings.HasSuffix(f, ".mm") &&
			!strings.HasSuffix(f, ".json") && !strings.HasSuffix(f, ".py") &&
			!strings.HasSuffix(f, ".md") && !strings.HasSuffix(f, "DEPS") &&
			!strings.HasSuffix(f, ".xtb") && !strings.HasSuffix(f, "grd") &&
			!strings.HasSuffix(f, ".ts") && !strings.HasSuffix(f, "gni") &&
			!strings.HasSuffix(f, ".gn") && !strings.HasSuffix(f, "txt") &&
			!strings.HasSuffix(f, ".py") && !strings.HasSuffix(f, "pyl") {
			filtered = append(filtered, f)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		lFileRank := 0
		rFileRank := 0
		if strings.HasSuffix(filtered[i], ".cc") {
			lFileRank++
		}
		if strings.HasSuffix(filtered[j], ".cc") {
			rFileRank++
		}
		return lFileRank > rFileRank
	})
	return filtered
}
