package dns

import (
	"bytes"
	"embed"
	"strings"
	"text/template"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v3/pkg/dns"
)

//go:embed templates/*
var templateFiles embed.FS

type (
	// RecordsetData represents a struct passed to recordset template
	RecordsetData struct {
		Zone           string
		ResourceFields map[string]string
		BlockName      string
		TfWorkPath     string
	}

	// ZoneData represents a struct passed to zone-creation template
	ZoneData struct {
		Zone                  string
		BlockName             string
		Type                  string
		Masters               []string
		Comment               string
		SignAndServe          bool
		SignAndServeAlgorithm string
		TsigKey               *dns.TSIGKey
		Target                string
		EndCustomerID         string
		TfWorkPath            string
	}

	// ImportData represents a struct passed to import script template
	ImportData struct {
		Zone          string
		ZoneConfigMap map[string]Types
		ResourceName  string
		TfWorkPath    string
	}
)

var funcs = template.FuncMap{
	"namedModulePath":           createNamedModulePath,
	"checkForResource":          checkForResource,
	"createUniqueRecordsetName": createUniqueRecordsetName,
}
var tmpl = template.Must(template.New("template").Funcs(funcs).ParseFS(templateFiles, "**/*.tmpl"))

func useTemplate(data interface{}, templateName string, trimBeginning bool) string {
	buf := bytes.Buffer{}

	if err := tmpl.Lookup(templateName).Execute(&buf, data); err != nil {
		return ""
	}

	res := string(buf.Bytes())

	if trimBeginning {
		res = strings.TrimLeft(res, "\n")
	}
	return res
}

// check if resource present in state
func checkForResource(rtype, name, tfWorkPath string) bool {

	if tfState == nil {
		if err := readTfState(tfWorkPath); err != nil {
			// not differentiating between not exists and file error
			return false
		}
	}
	for _, r := range tfState.Resources {
		if r.Type == rtype && r.Name == name {
			return true
		}
	}

	return false
}
