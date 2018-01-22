package data_exporter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/naveego/data_exporter"
)

var _ = Describe("MongoQuery", func() {

	It("should parse query into pipeline", func() {
		query := `[
			{"$match": { "_id": { "$in": ["mergeprocessingservice","streams.masterdata"] } } },
			{"$sort": {"_id":-1}},
			{"$group": { _id:"$item", stream:{"$first":"$c"}, pointer:{"$last":"$c"}}},
			{"$project": { "lag": { "$subtract": [  "$stream", "$pointer"]}}}
			]`
		Expect(ParseMongoAggregationPipeline(query)).ToNot(BeNil())

	})

})
