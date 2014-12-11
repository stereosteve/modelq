package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

type _CodeResult struct {
	name string
	err  error
}

type _CodeConfig struct {
	packageName    string
	touchTimestamp bool
}

func generateModels(dbName string, dbSchema _DbSchema, config _CodeConfig) {
	if fs, err := os.Stat(config.packageName); err != nil || !fs.IsDir() {
		os.Mkdir(config.packageName, os.ModeDir|os.ModePerm)
	}

	jobs := make(chan _CodeResult)
	for tbl, cols := range dbSchema {
		go func(tableName string, schema _TableSchema) {
			err := generateModel(dbName, tableName, schema, config)
			jobs <- _CodeResult{tableName, err}
		}(tbl, cols)
	}

	for i := 0; i < len(dbSchema); i++ {
		result := <-jobs
		if result.err != nil {
			log.Printf("Error when generating code for %s, %s", result.name, result.err)
		} else {
			log.Printf("Code generated for table %s, into package %s/%s.go", result.name, config.packageName, result.name)
		}
	}
	close(jobs)
}

func generateModel(dbName, tName string, schema _TableSchema, config _CodeConfig) error {
	file, err := os.Create(path.Join(config.packageName, tName+".go"))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(file)

	defer func() {
		w.Flush()
		file.Close()
	}()

	if err := writeCodeHeader(w, dbName, tName, config.packageName); err != nil {
		return errors.New(fmt.Sprintf("[%s] Error when writing the header into file.", tName))
	}
	if err := writeStruct(w, tName, schema); err != nil {
		return errors.New(fmt.Sprintf("[%s] Error when generating model struct into file.", tName))
	}
	if err := writeCodeFooter(w); err != nil {
		return errors.New(fmt.Sprintf("[%s] Error when generating footer into file.", tName))
	}

	return nil
}

func writeCodeHeader(w *bufio.Writer, dbName, tName, pName string) error {
	tmpl := `// Code generated by ModelQ, %s
// %s.go contains model for the database table [%s.%s]

package %s

import (
	"time"
	"github.com/mijia/modelq/gmq"
	"database/sql"
)`
	data := fmt.Sprintf(tmpl, time.Now().Format("2006-01-02 15:04"), tName, dbName, tName, pName)
	_, err := w.WriteString(data + "\n\n")
	return err
}

func writeCodeFooter(w *bufio.Writer) error {
	tmpl := `// just to bypass the golang import check
var _ = time.Now
var _ sql.DB
var _ gmq.OptionInt
`
	_, err := w.WriteString(tmpl + "\n\n")
	return err
}

func writeStruct(w *bufio.Writer, name string, schema _TableSchema) error {
	typeName := toCapitalCase(name)
	fieldTmpl := "\t%s %s `json:\"%s\"`%s"
	fields := make([]string, len(schema))
	for i, c := range schema {
		name := toCapitalCase(c.colName)
		fieldTypes := kFieldTypes
		// if c.isNullable {
		// 	fieldTypes = kNullFieldTypes
		// }
		fieldType, ok := fieldTypes[strings.ToLower(c.dataType)]
		if !ok {
			fieldType = "string"
		}
		comment := ""
		if c.comment != "" {
			comment = " // " + c.comment
		}
		fields[i] = fmt.Sprintf(fieldTmpl, name, fieldType, c.colName, comment)
	}
	structTmpl := `type %s struct {
%s
}`
	data := fmt.Sprintf(structTmpl, typeName, strings.Join(fields, "\n"))
	_, err := w.WriteString(data + "\n")
	return err
}

var (
	kFieldTypes map[string]string
	kNullFieldTypes map[string]string
)

func init() {
	kFieldTypes = map[string]string{
		"bigint": "int64",
		"int": "int",
		"tinyint": "int",
		"char": "string",
		"varchar": "string",
		"datetime": "time.Time",
		"decimal": "float64",
	}
	kNullFieldTypes = map[string]string{
		"bigint": "gmq.OptionInt64",
		"int": "gmq.OptionInt",
		"tinyint": "gmq.OptionInt",
		"char": "gmq.OptionString",
		"varchar": "gmq.OptionString",
		"datetime": "gmq.OptionTime",
		"decimal": "gmq.OptionFloat64",
	}
}

func toCapitalCase(name string) string {
	// cp___hello_12jiu -> CpHello12Jiu
	data := []byte(name)
	segStart := true
	endPos := 0
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			if segStart {
				if ch >= 'a' && ch <= 'z' {
					ch = ch - 'a' + 'A'
				}
				segStart = false
			} else {
				if ch >= 'A' && ch <= 'Z' {
					ch = ch - 'A' + 'a'
				}
			}
			data[endPos] = ch
			endPos++
		} else if ch >= '0' && ch <= '9' {
			data[endPos] = ch
			endPos++
			segStart = true
		} else {
			segStart = true
		}
	}
	return string(data[:endPos])
}
