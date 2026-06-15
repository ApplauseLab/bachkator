package config

const (
	FactoryPhasePlan      = "plan"
	FactoryPhaseImplement = "implement"
	FactoryPhaseMerge     = "merge"
)

func FactoryPhaseDeploy(name string) string {
	return "deploy." + name
}

func FactoryPhaseVerify(name string) string {
	return "verify." + name
}
