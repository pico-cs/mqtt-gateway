package devices

import (
	"fmt"
	"html/template"
)

const idxHTML = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>locos</title>
	</head>
	<body>
		<div><a href='/cs'>comand stations</a></div>
		<div><a href='/loco'>locos</a></div>
	</body>
</html>`

const csIdxHTML = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>command stations</title>
	</head>
	<body>
		<ul>
		{{range $k, $v := .CSMap -}}
			<li><div><a href='/cs/{{ $k }}'>{{ $k }}</a></div></li>
			<ul>
				<li>primary locos</li>
					<ul>
					{{range $k1, $v1 := $v.Primaries -}}
						<li><div><a href='/loco/{{ $k1 }}'>{{ $k1 }}</a></div></li>
					{{end -}}
					</ul>
				<li>secondary locos</li>
					<ul>
					{{range $k1, $v1 := $v.Secondaries -}}
						<li><div><a href='/loco/{{ $k1 }}'>{{ $k1 }}</a></div></li>
					{{end -}}
					</ul>
			</ul>
		{{end -}}
		</ul>
	</body>
</html>`

const locoIdxHTML = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>locos</title>
	</head>
	<body>
		<ul>
		{{range $k, $v := .LocoMap -}}
			<li><div><a href='/loco/{{ $k }}'>{{ $k }}</a></div></li>
		{{end -}}
		</ul>
	</body>
</html>`

var (
	csIdxTpl   *template.Template
	locoIdxTpl *template.Template
)

type csTpl struct {
	Primaries   map[string]*Loco
	Secondaries map[string]*Loco
}

type csTplData struct {
	CSMap map[string]csTpl
}

type locoTplData struct {
	LocoMap map[string]*Loco
}

func init() {
	var err error
	if csIdxTpl, err = template.New("csPage").Parse(csIdxHTML); err != nil {
		panic(fmt.Sprintf("template parse error %s", err))
	}
	if locoIdxTpl, err = template.New("locoPage").Parse(locoIdxHTML); err != nil {
		panic(fmt.Sprintf("template parse error %s", err))
	}
}
