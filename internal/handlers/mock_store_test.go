package handlers_test

import (
	"context"
	"errors"
)

var errMock = errors.New("mock error")

const testURL = "https://example.com"

// mockStore is a test double for URLRepository that can be configured to return errors.
type mockStore struct {
	saveErr             error
	getErr              error
	saveWithHashErr     error
	getCodeByHashErr    error
	savedCode           string
	savedURL            string
	savedHash           string
	getCodeByHashResult string
}

func (m *mockStore) Save(_ context.Context, code, url string) error {
	m.savedCode = code
	m.savedURL = url

	return m.saveErr
}

func (m *mockStore) Get(_ context.Context, _ string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}

	return testURL, nil
}

func (m *mockStore) SaveWithHash(_ context.Context, code, url, hash string) error {
	m.savedCode = code
	m.savedURL = url
	m.savedHash = hash

	return m.saveWithHashErr
}

func (m *mockStore) GetCodeByHash(_ context.Context, _ string) (string, error) {
	if m.getCodeByHashErr != nil {
		return "", m.getCodeByHashErr
	}

	return m.getCodeByHashResult, nil
}
