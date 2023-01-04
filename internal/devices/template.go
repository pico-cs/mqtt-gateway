package devices

import (
	"fmt"
	"html/template"
	"strings"
)

var indent = strings.Repeat(" ", 4)

const idxHTML = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>{{.Title}}</title>
	</head>
	<body>
		{{range $k, $v := .Items}}<div><a href='{{ $v }}'>{{ $k }}</a></div>{{end}}
	</body>
</html>`

var idxTpl *template.Template

func init() {
	var err error
	idxTpl, err = template.New("idxpage").Parse(idxHTML)
	if err != nil {
		panic(fmt.Sprintf("template parse error %s", err))
	}
}
