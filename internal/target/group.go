package target

import "github.com/applauselab/bachkator/internal/model"

func groupHandler() TargetHandler {
	return compositeTargetHandler{
		targetType:     model.TargetTypeGroup,
		label:          "group",
		separator:      ", ",
		fingerprintKey: "targets",
		childKind:      "group_member",
		children:       groupTargets,
	}
}

func groupTargets(body model.TargetBody) ([]string, bool) {
	group, ok := body.(model.GroupSpec)
	return group.Targets, ok
}
