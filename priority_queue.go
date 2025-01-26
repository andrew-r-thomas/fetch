package fetch

type PQ []*PQItem
type PQItem struct {
	qId      int
	priority int
	freq     int
	size     int
}

func (pq PQ) Len() int           { return len(pq) }
func (pq PQ) Less(a, b int) bool { return pq[a].priority < pq[b].priority }
func (pq PQ) Swap(a, b int) {
	pq[a], pq[b] = pq[b], pq[a]
	pq[a].qId = a
	pq[b].qId = b
}
func (pq *PQ) Push(x any) {
	n := len(*pq)
	item := x.(*PQItem)
	item.qId = n
	*pq = append(*pq, item)
}
func (pq *PQ) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.qId = -1
	*pq = old[0 : n-1]
	return item
}
