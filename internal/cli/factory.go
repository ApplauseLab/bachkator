package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newFactoryCommand(
	deps Dependencies,
	opts *options,
	stdout io.Writer,
	stderr io.Writer,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "factory",
		Short: "Manage Factory Work Items",
	}

	submitCmd := &cobra.Command{
		Use:   "submit <factory>",
		Short: "Submit a Factory Work Item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactorySubmit(
				makeFactoryContext(cmd.Context(), deps, opts, stdout, stderr),
				args,
			)
		},
	}
	bindFactorySubmitFlags(submitCmd, opts)
	cmd.AddCommand(submitCmd)

	listCmd := &cobra.Command{
		Use:   "list <factory>",
		Short: "List Factory Work Items",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactoryList(
				makeFactoryContext(cmd.Context(), deps, opts, stdout, stderr),
				args,
			)
		},
	}
	bindFactoryListFlags(listCmd, opts)
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "inspect <factory> <work-item-id>",
		Short: "Inspect a Factory Work Item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactoryInspect(
				makeFactoryContext(cmd.Context(), deps, opts, stdout, stderr),
				args,
			)
		},
	})

	cancelCmd := &cobra.Command{
		Use:   "cancel <factory> <work-item-id>",
		Short: "Cancel a Factory Work Item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactoryCancel(
				makeFactoryContext(cmd.Context(), deps, opts, stdout, stderr),
				args,
			)
		},
	}
	bindFactoryCancelFlags(cancelCmd, opts)
	cmd.AddCommand(cancelCmd)

	approveCmd := &cobra.Command{
		Use:   "approve <factory> <work-item-id>",
		Short: "Approve a gated Factory phase",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactoryApprove(
				makeFactoryContext(cmd.Context(), deps, opts, stdout, stderr),
				args,
			)
		},
	}
	bindFactoryApproveFlags(approveCmd, opts)
	cmd.AddCommand(approveCmd)

	startCmd := &cobra.Command{
		Use:   "start <factory>",
		Short: "Start a Factory daemon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactoryStart(
				makeFactoryContext(cmd.Context(), deps, opts, stdout, stderr),
				args,
			)
		},
	}
	bindExecutionFlags(startCmd, opts)
	bindFactoryStartFlags(startCmd, opts)
	cmd.AddCommand(startCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "status <factory>",
		Short: "Show Factory daemon status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFactoryStatus(
				makeFactoryContext(cmd.Context(), deps, opts, stdout, stderr),
				args,
			)
		},
	})

	return cmd
}

func makeFactoryContext(
	ctx context.Context,
	deps Dependencies,
	opts *options,
	stdout io.Writer,
	stderr io.Writer,
) commandContext {
	return commandContext{
		context: ctx,
		project: projectFromContext(ctx),
		deps:    deps,
		opts:    opts,
		stdout:  stdout,
		stderr:  stderr,
	}
}

func runFactorySubmit(ctx commandContext, args []string) error {
	if ctx.deps.FactorySubmit == nil {
		return fmt.Errorf("factory submit dependency is not configured")
	}
	if len(args) == 0 {
		return UsageErrorf("usage: bach factory submit <factory> --title <title>")
	}
	if len(args) != 1 {
		return fmt.Errorf("unexpected factory submit argument %q", args[1])
	}
	factoryName := args[0]
	opts := FactorySubmitOptions{
		Workflow:  ctx.opts.factoryWorkflow,
		Title:     ctx.opts.factoryTitle,
		Body:      ctx.opts.factoryBody,
		BodyFile:  ctx.opts.factoryBodyFile,
		Priority:  ctx.opts.factoryPriority,
		Labels:    append([]string(nil), ctx.opts.factoryLabels...),
		DedupeKey: ctx.opts.factoryDedupeKey,
		Plan:      ctx.opts.factoryPlan,
	}
	result, err := ctx.deps.FactorySubmit(ctx.context, ctx.project, factoryName, opts)
	if err != nil {
		return err
	}
	if ctx.opts.json {
		return writeFactoryJSON(ctx.stdout, factorySubmitView{
			Item:    factoryWorkItemViewFor(result.Item),
			Created: result.Created,
		})
	}
	status := "submitted"
	if !result.Created {
		status = "deduped"
	}
	_, err = fmt.Fprintf(
		ctx.stdout,
		"%s %s factory=%s workflow=%s status=%s evidence=%s\n",
		status,
		result.Item.ID,
		result.Item.Factory,
		result.Item.Workflow,
		result.Item.Lifecycle,
		result.Item.IntakeEvidenceURI,
	)
	return err
}

func runFactoryList(ctx commandContext, args []string) error {
	if ctx.deps.FactoryList == nil {
		return fmt.Errorf("factory list dependency is not configured")
	}
	if len(args) == 0 {
		return UsageErrorf("usage: bach factory list <factory>")
	}
	if len(args) != 1 {
		return fmt.Errorf("unexpected factory list argument %q", args[1])
	}
	factoryName := args[0]
	status := ctx.opts.factoryStatus
	opts := FactoryListOptions{
		Workflow: ctx.opts.factoryWorkflow,
		Status:   status,
	}
	items, err := ctx.deps.FactoryList(ctx.context, ctx.project, factoryName, opts)
	if err != nil {
		return err
	}
	if ctx.opts.json {
		views := make([]factoryWorkItemView, 0, len(items))
		for _, item := range items {
			views = append(views, factoryWorkItemViewFor(item))
		}
		return writeFactoryJSON(ctx.stdout, factoryListView{Items: views})
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(
			ctx.stdout,
			"%s %-10s %-12s %-16s created=%s title=%s\n",
			item.ID,
			item.Lifecycle,
			item.Workflow,
			item.Priority,
			formatFactoryTime(item.CreatedAt),
			item.Title,
		); err != nil {
			return err
		}
	}
	return nil
}

func runFactoryInspect(ctx commandContext, args []string) error {
	if ctx.deps.FactoryInspect == nil {
		return fmt.Errorf("factory inspect dependency is not configured")
	}
	if len(args) < 2 {
		return UsageErrorf("usage: bach factory inspect <factory> <work-item-id>")
	}
	if len(args) != 2 {
		return fmt.Errorf("unexpected factory inspect argument %q", args[2])
	}
	factoryName := args[0]
	workItemID := args[1]
	item, err := ctx.deps.FactoryInspect(ctx.context, ctx.project, factoryName, workItemID)
	if err != nil {
		return err
	}
	if ctx.opts.json {
		return writeFactoryJSON(ctx.stdout, factoryWorkItemViewFor(item))
	}
	return formatFactoryInspection(ctx.stdout, item)
}

func runFactoryCancel(ctx commandContext, args []string) error {
	if ctx.deps.FactoryCancel == nil {
		return fmt.Errorf("factory cancel dependency is not configured")
	}
	if len(args) < 2 {
		return UsageErrorf("usage: bach factory cancel <factory> <work-item-id> --reason <text>")
	}
	if len(args) != 2 {
		return fmt.Errorf("unexpected factory cancel argument %q", args[2])
	}
	factoryName := args[0]
	workItemID := args[1]
	opts := FactoryCancelOptions{Reason: ctx.opts.factoryReason}
	item, err := ctx.deps.FactoryCancel(ctx.context, ctx.project, factoryName, workItemID, opts)
	if err != nil {
		return err
	}
	if ctx.opts.json {
		return writeFactoryJSON(ctx.stdout, factoryWorkItemViewFor(item))
	}
	_, err = fmt.Fprintf(
		ctx.stdout,
		"cancelled %s factory=%s workflow=%s reason=%s\n",
		item.ID,
		item.Factory,
		item.Workflow,
		item.CancelReason,
	)
	return err
}

func runFactoryApprove(ctx commandContext, args []string) error {
	if ctx.deps.FactoryApprove == nil {
		return fmt.Errorf("factory approve dependency is not configured")
	}
	if len(args) < 2 {
		return UsageErrorf("usage: bach factory approve <factory> <work-item-id> --phase <phase>")
	}
	if len(args) != 2 {
		return fmt.Errorf("unexpected factory approve argument %q", args[2])
	}
	factoryName := args[0]
	workItemID := args[1]
	if ctx.opts.factoryPhase == "" {
		return UsageErrorf("--phase is required")
	}
	opts := FactoryApproveOptions{
		Phase:  ctx.opts.factoryPhase,
		Reason: ctx.opts.factoryReason,
	}
	result, err := ctx.deps.FactoryApprove(ctx.context, ctx.project, factoryName, workItemID, opts)
	if err != nil {
		return err
	}
	if ctx.opts.json {
		return writeFactoryJSON(ctx.stdout, factoryApproveView{
			Approval: factoryApprovalViewFor(result.Approval),
			Existing: result.Existing,
		})
	}
	verb := "approved"
	if result.Existing {
		verb = "approval already exists for"
	}
	_, err = fmt.Fprintf(
		ctx.stdout,
		"%s %s factory=%s workflow=%s phase=%s approval=%s\n",
		verb,
		result.Approval.WorkItemID,
		result.Approval.Factory,
		result.Approval.Workflow,
		result.Approval.Phase,
		result.Approval.ID,
	)
	return err
}

func runFactoryStart(ctx commandContext, args []string) error {
	if ctx.deps.FactoryStart == nil {
		return fmt.Errorf("factory start dependency is not configured")
	}
	if len(args) == 0 {
		return UsageErrorf("usage: bach factory start <factory>")
	}
	if len(args) != 1 {
		return fmt.Errorf("unexpected factory start argument %q", args[1])
	}
	stdout := ctx.stdout
	if ctx.opts.json {
		stdout = io.Discard
	}
	result, err := ctx.deps.FactoryStart(ctx.context, ctx.project, args[0], FactoryStartOptions{
		Yes:           ctx.opts.yes,
		Force:         ctx.opts.force,
		LogOnly:       ctx.opts.logOnly,
		Verbose:       ctx.opts.verbose,
		Parallelism:   ctx.opts.jobs,
		PollInterval:  ctx.opts.factoryPollInterval,
		RenewInterval: ctx.opts.factoryRenewInterval,
		LeaseTTL:      ctx.opts.factoryLeaseTTL,
		Stdout:        stdout,
		Stderr:        ctx.stderr,
	})
	if err != nil {
		return err
	}
	if ctx.opts.json {
		return writeFactoryJSON(ctx.stdout, factoryStartView{
			DaemonID: result.DaemonID,
			Lease:    factoryDaemonLeaseViewFor(result.Lease),
		})
	}
	_, err = fmt.Fprintf(
		ctx.stdout,
		"factory daemon %s stopped factory=%s\n",
		result.DaemonID,
		result.Lease.Factory,
	)
	return err
}

func runFactoryStatus(ctx commandContext, args []string) error {
	if ctx.deps.FactoryStatus == nil {
		return fmt.Errorf("factory status dependency is not configured")
	}
	if len(args) == 0 {
		return UsageErrorf("usage: bach factory status <factory>")
	}
	if len(args) != 1 {
		return fmt.Errorf("unexpected factory status argument %q", args[1])
	}
	result, err := ctx.deps.FactoryStatus(ctx.context, ctx.project, args[0])
	if err != nil {
		return err
	}
	view := factoryStatusViewFor(result.Status)
	if ctx.opts.json {
		return writeFactoryJSON(ctx.stdout, view)
	}
	if view.Lease.DaemonID == "" {
		_, err = fmt.Fprintf(ctx.stdout, "factory=%s daemon=none\n", args[0])
		return err
	}
	_, err = fmt.Fprintf(
		ctx.stdout,
		"factory=%s daemon=%s status=%s expires=%s active_item=%s counts=%v\n",
		view.Lease.Factory,
		view.Lease.DaemonID,
		view.Lease.Status,
		view.Lease.ExpiresAt,
		view.ActiveItemID,
		view.LifecycleCounts,
	)
	return err
}
