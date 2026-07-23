package model

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/lamxy/fiberhouse/example_application/module/command-module/entity"
	"gorm.io/gorm/schema"
)

func TestExampleRecordSchema(t *testing.T) {
	t.Parallel()

	if got := (entity.ExampleRecord{}).TableName(); got != "example_records" {
		t.Fatalf("TableName() = %q, want example_records", got)
	}

	parsed, err := schema.Parse(&entity.ExampleRecord{}, &syncMap, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	tests := map[string][]string{
		"ID":          {"PRIMARYKEY", "AUTOINCREMENT"},
		"Name":        {"SIZE:80", "NOT NULL", "UNIQUEINDEX:UX_EXAMPLE_RECORDS_NAME"},
		"Description": {"SIZE:500", "NOT NULL", "DEFAULT:''"},
		"Status":      {"SIZE:16", "NOT NULL", "INDEX:IDX_EXAMPLE_RECORDS_STATUS_CREATED"},
		"CreatedAt":   {"INDEX:IDX_EXAMPLE_RECORDS_STATUS_CREATED,SORT:DESC"},
	}
	recordType := reflect.TypeOf(entity.ExampleRecord{})
	for fieldName, fragments := range tests {
		field, ok := recordType.FieldByName(fieldName)
		if !ok {
			t.Fatalf("missing field %s", fieldName)
		}
		tag := strings.ToUpper(field.Tag.Get("gorm"))
		for _, fragment := range fragments {
			if !strings.Contains(tag, fragment) {
				t.Errorf("%s gorm tag %q does not contain %q", fieldName, tag, fragment)
			}
		}
		if parsed.FieldsByName[fieldName] == nil {
			t.Errorf("%s missing from parsed GORM schema", fieldName)
		}
	}
}

var syncMap = sync.Map{}
