package scansion

import (
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
)

const (
	dbTagName     = "db"
	dbTagOptionPk = "pk"

	scanPrefix = "scan:"
)

type fieldMapEntry struct {
	Type      reflect.Type
	Value     reflect.Value
	StructIdx int
	Optional  bool
}

type fieldMapType map[string]fieldMapEntry

func getFieldMap(s interface{}) (fieldMapType, error) {
	sVal := reflect.ValueOf(s)
	sType := reflect.TypeOf(s)
	if sType.Kind() != reflect.Pointer && sType.Elem().Kind() != reflect.Struct {
		return nil, errors.New("input is not a struct pointer")
	}

	rootMapEntry := fieldMapEntry{
		Type:  sType,
		Value: sVal,
	}

	fieldMap, err := getFieldMapHelper(s, nil, []reflect.Type{sType}, false)
	if err != nil {
		return fieldMapType{}, err
	}

	fieldMap[""] = rootMapEntry

	return fieldMap, nil
}

func getFieldMapHelper(s interface{}, path []string, visited []reflect.Type, optional bool) (fieldMapType, error) {
	fieldMap := make(fieldMapType)

	sType := reflect.TypeOf(s).Elem()
	sValue := reflect.ValueOf(s).Elem()
	if sType.Kind() == reflect.Slice {
		sType = sType.Elem()
		sValue = reflect.New(sType).Elem()
	}

	for i := 0; i < sType.NumField(); i++ {
		structField := sType.Field(i)
		fullDbTag := structField.Tag.Get(dbTagName)
		if fullDbTag == "" {
			continue
		}
		dbTagParts := strings.Split(fullDbTag, ",")
		dbTagParts = mapFn(dbTagParts, strings.TrimSpace)

		dbFieldName := fullDbTag
		if len(dbTagParts) > 1 {
			dbFieldName = dbTagParts[0]
		}

		if structField.Type.Kind() == reflect.Slice {
			visitedType := structField.Type
			if visitedType.Kind() == reflect.Slice || visitedType.Kind() == reflect.Pointer {
				visitedType = visitedType.Elem()
			}
			if slices.Contains(path, dbFieldName) || slices.Contains(visited, visitedType) {
				continue
			}

			nestedMap, err := getFieldMapHelper(
				reflect.New(visitedType).Interface(),
				append(path, dbFieldName),
				append(visited, visitedType),
				optional)
			if err != nil {
				return nil, err
			}
			maps.Copy(fieldMap, nestedMap)
		} else if structField.Type.Kind() == reflect.Pointer &&
			structField.Type.Elem().Kind() == reflect.Struct {
			visitedType := structField.Type.Elem()

			if slices.Contains(path, dbFieldName) || slices.Contains(visited, visitedType) {
				continue
			}

			nestedMap, err := getFieldMapHelper(
				reflect.New(visitedType).Interface(),
				append(path, dbFieldName),
				append(visited, visitedType),
				true)
			if err != nil {
				return nil, err
			}
			maps.Copy(fieldMap, nestedMap)
		}

		scopedName := strings.Join(append(path, dbFieldName), ".")
		fieldMap[scopedName] = fieldMapEntry{
			Type:      structField.Type,
			Value:     sValue.Field(i),
			StructIdx: i,
			Optional:  optional,
		}
	}

	return fieldMap, nil
}

func getPkValue(s reflect.Value) (reflect.Value, error) {
	var pkValue reflect.Value

	if s.Kind() == reflect.Pointer {
		s = s.Elem()
	}

	if s.Kind() != reflect.Struct {
		return reflect.Value{}, errors.New("input must be of type struct")
	}

	for i := 0; i < s.NumField(); i++ {
		fieldType := s.Type().Field(i)
		fieldVal := s.Field(i)
		fullDbTag := fieldType.Tag.Get(dbTagName)
		if fullDbTag == "" {
			continue
		}
		dbTagParts := strings.Split(fullDbTag, ",")
		dbTagParts = mapFn(dbTagParts, strings.TrimSpace)

		if len(dbTagParts) == 2 && dbTagParts[1] == dbTagOptionPk {
			if pkValue.IsValid() {
				return reflect.Value{}, errors.New("exactly one column must have 'pk' set")
			}
			pkValue = fieldVal
		}
	}

	if !pkValue.IsValid() {
		return reflect.Value{}, errors.New("exactly one column must have 'pk' set")
	}

	return pkValue, nil
}
