package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestFieldMapper(t *testing.T) {

	type expectedResult struct {
		fm map[string]int
		e  error
	}

	testCases := []struct {
		headerSlice    []string
		expectedResult map[string]int
		err            error
	}{
		{[]string{"ID", "Enabled", "RepositoryID", "ClassificationID", "CollectionIdentifier", "Title", "SortTitle"}, map[string]int{"ID": 0, "Title": 5, "CollectionIdentifier": 4}, nil},
		{[]string{"ID", "CollectionIdentifier", "Title", "SortTitle"}, map[string]int{"ID": 0, "Title": 2, "CollectionIdentifier": 1}, nil},
		{[]string{"id", "collectionidentifier", "title"}, map[string]int{"ID": 0, "CollectionIdentifier": 1, "Title": 2}, nil},
		{[]string{"CollectionIdentifier", "Title"}, nil, errors.New("no ID column found in CSV")},
	}

	for _, test := range testCases {
		fmap, err := fieldMapper(test.headerSlice)
		if err != nil {
			if err.Error() != test.err.Error() {
				t.Errorf("The fieldMapper function returned the following err: %v.", err)
			}
		}
		mEq := reflect.DeepEqual(fmap, test.expectedResult)
		if !mEq {
			t.Errorf("Expected %v but returned %v.\n", test.expectedResult, fmap)
		}
	}

}

func TestGenerateFilename(t *testing.T) {
	testCases := []struct {
		archonCollection
		nameFlag       string
		expectedResult string
	}{
		{archonCollection{ID: 1, CollectionIdentifier: "VF00001", Title: "Some Title $"}, "Title", "Some_Title___1"},
		{archonCollection{ID: 1, CollectionIdentifier: "VF00001", Title: "Some Title $"}, "CollectionIdentifier", "VF00001_1"},
		{archonCollection{ID: 1, CollectionIdentifier: "VF00001", Title: "Some Title $"}, "CollectionIdentifier,Title", "VF00001__Some_Title___1"},
		{archonCollection{ID: 1, CollectionIdentifier: "VF00001", Title: "Some Title ($!)"}, "Title,CollectionIdentifier", "Some_Title_______VF00001_1"},
		{archonCollection{ID: 1, CollectionIdentifier: "VF00001", Title: "Some Title $"}, "", "ead_1"},
		{archonCollection{ID: 1, CollectionIdentifier: "VF00001"}, "Title", "ead_1"},
		{archonCollection{ID: 1}, "CollectionIdentifier", "ead_1"},
	}

	for _, test := range testCases {
		fn := generateFilename(test.archonCollection, test.nameFlag)
		if fn != test.expectedResult {
			t.Errorf("Expected %s but returned %s.\n", test.expectedResult, fn)
		}
	}
}

func TestAddURL(t *testing.T) {
	testCases := []struct {
		archonCollection
		host           string
		expectedResult string
	}{
		{archonCollection{ID: 1, CollectionIdentifier: "VF00001", Title: "Some Title $"}, "https://beckerarchives.wustl.edu", "https://beckerarchives.wustl.edu/index.php?p=collections/ead&templateset=ead&disabletheme=1&id=1"},
		{archonCollection{ID: 2, CollectionIdentifier: "VF00002", Title: "Some Title $"}, "http://127.0.0.1", "http://127.0.0.1/index.php?p=collections/ead&templateset=ead&disabletheme=1&id=2"},
		{archonCollection{ID: 3, CollectionIdentifier: "VF00003", Title: "Some Title $"}, "http://192.168.1.95:8000", "http://192.168.1.95:8000/index.php?p=collections/ead&templateset=ead&disabletheme=1&id=3"},
		{archonCollection{ID: 4, CollectionIdentifier: "VF00004", Title: "Some Title ($!)"}, "http://localhost", "http://localhost/index.php?p=collections/ead&templateset=ead&disabletheme=1&id=4"},
	}
	for _, test := range testCases {
		tURL := addURL(test.host, test.archonCollection)
		if tURL != test.expectedResult {
			t.Errorf("Expected %s but returned %s.\n", test.expectedResult, tURL)
		}
	}
}
