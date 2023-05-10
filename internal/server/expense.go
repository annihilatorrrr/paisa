package server

import (
	"math"

	"github.com/ananthakumaran/paisa/internal/model/posting"
	"github.com/ananthakumaran/paisa/internal/query"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type Node struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type Link struct {
	Source uint    `json:"source"`
	Target uint    `json:"target"`
	Value  float64 `json:"value"`
}

type Pair struct {
	Source uint `json:"source"`
	Target uint `json:"target"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

func GetExpense(db *gorm.DB) gin.H {
	expenses := query.Init(db).Like("Expenses:%").NotLike("Expenses:Tax").All()
	incomes := query.Init(db).Like("Income:%").All()
	investments := query.Init(db).Like("Assets:%").NotLike("Assets:Checking").All()
	taxes := query.Init(db).Like("Expenses:Tax").All()
	postings := query.Init(db).All()

	graph := make(map[string]Graph)
	for fy, ps := range posting.GroupByFY(postings) {
		graph[fy] = computeGraph(ps)
	}

	return gin.H{
		"expenses": expenses,
		"month_wise": gin.H{
			"expenses":    posting.GroupByMonth(expenses),
			"incomes":     posting.GroupByMonth(incomes),
			"investments": posting.GroupByMonth(investments),
			"taxes":       posting.GroupByMonth(taxes)},
		"year_wise": gin.H{
			"expenses":    posting.GroupByFY(expenses),
			"incomes":     posting.GroupByFY(incomes),
			"investments": posting.GroupByFY(investments),
			"taxes":       posting.GroupByFY(taxes)},
		"graph": graph}
}

func computeGraph(postings []posting.Posting) Graph {
	nodes := make(map[string]Node)
	links := make(map[Pair]float64)

	var nodeID uint = 0

	grouped := lo.GroupBy(postings, func(p posting.Posting) string { return p.TransactionID })
	transactions := lo.Map(lo.Values(grouped), func(ps []posting.Posting, _ int) Transaction {
		sample := ps[0]
		return Transaction{ID: sample.TransactionID, Date: sample.Date, Payee: sample.Payee, Postings: ps}
	})

	for _, p := range postings {
		_, ok := nodes[p.Account]
		if !ok {
			nodeID++
			nodes[p.Account] = Node{ID: nodeID, Name: p.Account}
		}

	}

	for _, t := range transactions {
		from := lo.Filter(t.Postings, func(p posting.Posting, _ int) bool { return p.Amount < 0 })
		to := lo.Filter(t.Postings, func(p posting.Posting, _ int) bool { return p.Amount > 0 })

		for _, f := range from {
			for math.Abs(f.Amount) > 0.1 && len(to) > 0 {
				top := to[0]
				if top.Amount > -f.Amount {
					links[Pair{Source: nodes[f.Account].ID, Target: nodes[top.Account].ID}] += -f.Amount
					top.Amount -= f.Amount
					f.Amount = 0
				} else {
					links[Pair{Source: nodes[f.Account].ID, Target: nodes[top.Account].ID}] += top.Amount
					f.Amount += top.Amount
					to = to[1:]
				}
			}
		}
	}

	return Graph{Nodes: lo.Values(nodes), Links: lo.Map(lo.Keys(links), func(k Pair, _ int) Link {
		return Link{Source: k.Source, Target: k.Target, Value: links[k]}
	})}

}