package domain

type Cluster struct {
	Name       string
	Instances  []Instance
	Collection Collection
}
