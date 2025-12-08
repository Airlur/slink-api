package model

import (
{{if .HasTimeField}}
	"time"
{{end}}
    "gorm.io/gorm"
)

type {{.StructName}} struct {
    gorm.Model
    {{- range .Columns}}
    {{pad .Name $.MaxNameLen}}{{pad .Type $.MaxTypeLen}}{{pad .FullTag $.MaxTagLen}} // {{.Comment}}
    {{- end}}
}

func ({{.StructName}}) TableName() string {
    return "{{.TableName}}"
}