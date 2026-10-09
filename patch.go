package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

type bookPatch struct {
	Title     *string
	Author    *string
	Year      *int
	Status    *Status
	Tags      []string
	Publisher *Publisher
}

func parseBookPatch(m map[string]any) (*bookPatch, error) {
	p := &bookPatch{}

	if v, ok := m["title"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("title must be string")
		}
		if s == "" {
			return nil, errors.New("title cannot be empty")
		}
		p.Title = &s
	}

	if v, ok := m["author"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("author must be string")
		}
		p.Author = &s
	}

	if v, ok := m["year"]; ok {
		n, ok := v.(json.Number)
		if !ok {
			return nil, errors.New("year must be number")
		}
		y, err := n.Int64()
		if err != nil {
			return nil, errors.New("year must be integer")
		}
		yi := int(y)
		p.Year = &yi
	}

	if v, ok := m["status"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("status must be string")
		}
		st := Status(s)
		switch st {
		case StatusAvailable, StatusBorrowed, StatusLost:
			p.Status = &st
		default:
			return nil, fmt.Errorf("unknown status: %q", s)
		}
	}

	if v, ok := m["tags"]; ok {
		arr, ok := v.([]any)
		if !ok {
			return nil, errors.New("tags must be array")
		}
		tags := make([]string, 0, len(arr))
		for _, item := range arr {
			s, ok := item.(string)
			if !ok {
				return nil, errors.New("tags must be string array")
			}
			tags = append(tags, s)
		}
		p.Tags = tags
	}

	if v, ok := m["publisher"]; ok {
		pub, ok := v.(map[string]any)
		if !ok {
			return nil, errors.New("publisher must be object")
		}
		p2 := &Publisher{}
		for key, val := range pub {
			s, ok := val.(string)
			if !ok {
				return nil, errors.New("publisher values must be string")
			}
			switch key {
			case "name":
				p2.Name = s
			case "country":
				p2.Country = s
			default:
				return nil, fmt.Errorf("unknown publisher field: %q", key)
			}
		}
		p.Publisher = p2
	}

	return p, nil
}
