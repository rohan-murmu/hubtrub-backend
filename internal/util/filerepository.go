package util

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

// FileRepository handles file operations for jsonl files
type FileRepository struct {
	filePath string
	mu       sync.RWMutex
}

// NewFileRepository creates a new file repository
func NewFileRepository(filePath string) *FileRepository {
	return &FileRepository{
		filePath: filePath,
	}
}

// ReadAll reads all records from jsonl file
func (fr *FileRepository) ReadAll(v interface{}) error {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	file, err := os.Open(fr.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Return empty if file doesn't exist
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	items := make([]interface{}, 0)

	for scanner.Scan() {
		var item map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			continue
		}
		items = append(items, item)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Convert to the target type
	data, _ := json.Marshal(items)
	return json.Unmarshal(data, v)
}

// Write appends a single record to jsonl file
func (fr *FileRepository) Write(record interface{}) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	file, err := os.OpenFile(fr.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	_, err = file.WriteString(string(data) + "\n")
	return err
}

// ReadByID reads a single record by ID
func (fr *FileRepository) ReadByID(id string, idField string, v interface{}) error {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	file, err := os.Open(fr.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var item map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			continue
		}

		if itemID, ok := item[idField].(string); ok && itemID == id {
			data, _ := json.Marshal(item)
			return json.Unmarshal(data, v)
		}
	}

	return os.ErrNotExist
}

// Update updates a record by ID
func (fr *FileRepository) Update(id string, idField string, updatedRecord interface{}) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	file, err := os.Open(fr.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	found := false

	for scanner.Scan() {
		var item map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			lines = append(lines, scanner.Text())
			continue
		}

		if itemID, ok := item[idField].(string); ok && itemID == id {
			data, _ := json.Marshal(updatedRecord)
			lines = append(lines, string(data))
			found = true
		} else {
			lines = append(lines, scanner.Text())
		}
	}

	if !found {
		return os.ErrNotExist
	}

	// Write back to file
	file, err = os.OpenFile(fr.filePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, line := range lines {
		file.WriteString(line + "\n")
	}

	return nil
}

// Delete deletes a record by ID
func (fr *FileRepository) Delete(id string, idField string) error {
	fr.mu.Lock()
	defer fr.mu.Unlock()

	file, err := os.Open(fr.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	found := false

	for scanner.Scan() {
		var item map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			lines = append(lines, scanner.Text())
			continue
		}

		if itemID, ok := item[idField].(string); ok && itemID == id {
			found = true
		} else {
			lines = append(lines, scanner.Text())
		}
	}

	if !found {
		return os.ErrNotExist
	}

	// Write back to file
	file, err = os.OpenFile(fr.filePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, line := range lines {
		file.WriteString(line + "\n")
	}

	return nil
}
