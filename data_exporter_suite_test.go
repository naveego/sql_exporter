package data_exporter_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSqlExporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SqlExporter Suite")
}
