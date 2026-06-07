package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
)

func (app App) moduleOwners(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("module owners requires a subcommand")
	}
	if isHelp(args[0]) {
		app.printHelp("module owners")
		return nil
	}
	switch args[0] {
	case "list":
		return app.moduleOwnersList(ctx, args[1:])
	case "add":
		return app.moduleOwnersAdd(ctx, args[1:])
	case "remove":
		return app.moduleOwnersRemove(ctx, args[1:])
	default:
		return fmt.Errorf("unknown module owners subcommand %q", args[0])
	}
}

func (app App) moduleOwnersList(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module owners list")
		return nil
	}
	flags := flag.NewFlagSet("module owners list", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("module owners list requires a module name")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	response, err := client.ListModuleOwners(ctx, flags.Arg(0))
	if err != nil {
		return fmt.Errorf("module owners list failed: %w", err)
	}
	printModuleOwners(app.output(), response)
	return nil
}

func (app App) moduleOwnersAdd(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module owners add")
		return nil
	}
	flags := flag.NewFlagSet("module owners add", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	subjectType := flags.String("subject-type", "", "subject type: user or team")
	subject := flags.String("subject", "", "user or team name")
	role := flags.String("role", "", "role: owner or maintainer")
	actor := flags.String("actor", "", "deprecated: only sent when provided and only works when server actor override is enabled")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("module owners add requires a module name")
	}
	if !validSubjectType(*subjectType) {
		return errors.New("module owners add requires --subject-type user|team")
	}
	if strings.TrimSpace(*subject) == "" {
		return errors.New("module owners add requires --subject")
	}
	if !validOwnerRole(*role) {
		return errors.New("module owners add requires --role owner|maintainer")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	owner, err := client.AddModuleOwner(ctx, flags.Arg(0), api.AddModuleOwnerRequest{
		SubjectType: strings.TrimSpace(*subjectType),
		Subject:     strings.TrimSpace(*subject),
		Role:        strings.TrimSpace(*role),
		Actor:       optionalActor(flags, *actor),
	})
	if err != nil {
		return governanceCommandError("module owners add", flagProvided(flags, "actor"), err)
	}
	fmt.Fprintf(app.output(), "Added %s %s as %s for %s (owner_id=%s)\n", owner.SubjectType, owner.Subject, owner.Role, owner.ModuleName, owner.ID)
	return nil
}

func (app App) moduleOwnersRemove(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module owners remove")
		return nil
	}
	flags := flag.NewFlagSet("module owners remove", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	ownerID := flags.String("owner-id", "", "owner id")
	actor := flags.String("actor", "", "deprecated: only sent when provided and only works when server actor override is enabled")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("module owners remove requires a module name")
	}
	if strings.TrimSpace(*ownerID) == "" {
		return errors.New("module owners remove requires --owner-id")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	if err := client.RemoveModuleOwner(ctx, flags.Arg(0), strings.TrimSpace(*ownerID), optionalActor(flags, *actor)); err != nil {
		return governanceCommandError("module owners remove", flagProvided(flags, "actor"), err)
	}
	fmt.Fprintf(app.output(), "Removed owner %s from %s\n", strings.TrimSpace(*ownerID), flags.Arg(0))
	return nil
}

func (app App) approvals(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("approvals requires a subcommand")
	}
	if isHelp(args[0]) {
		app.printHelp("approvals")
		return nil
	}
	switch args[0] {
	case "request":
		return app.approvalsRequest(ctx, args[1:])
	case "status":
		return app.approvalsStatus(ctx, args[1:])
	case "approve":
		return app.approvalsDecide(ctx, args[1:], true)
	case "reject":
		return app.approvalsDecide(ctx, args[1:], false)
	default:
		return fmt.Errorf("unknown approvals subcommand %q", args[0])
	}
}

func (app App) approvalsRequest(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("approvals request")
		return nil
	}
	flags := flag.NewFlagSet("approvals request", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	reportID := flags.String("report-id", "", "breaking report id")
	actor := flags.String("actor", "", "deprecated: only sent when provided and only works when server actor override is enabled")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if strings.TrimSpace(*reportID) == "" {
		return errors.New("approvals request requires --report-id")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	request, err := client.CreateApprovalRequest(ctx, strings.TrimSpace(*reportID), api.CreateApprovalRequestRequest{Actor: optionalActor(flags, *actor)})
	if err != nil {
		return governanceCommandError("approvals request", flagProvided(flags, "actor"), err)
	}
	printApprovalRequest(app.output(), request)
	return nil
}

func (app App) approvalsStatus(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("approvals status")
		return nil
	}
	flags := flag.NewFlagSet("approvals status", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	reportID := flags.String("report-id", "", "breaking report id")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if strings.TrimSpace(*reportID) == "" {
		return errors.New("approvals status requires --report-id")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	request, err := client.GetApprovalStatus(ctx, strings.TrimSpace(*reportID))
	if err != nil {
		return fmt.Errorf("approvals status failed: %w", err)
	}
	printApprovalRequest(app.output(), request)
	return nil
}

func (app App) approvalsDecide(ctx context.Context, args []string, approve bool) error {
	command := "approvals approve"
	if !approve {
		command = "approvals reject"
	}
	if hasHelp(args) {
		app.printHelp(command)
		return nil
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	requestID := flags.String("request-id", "", "approval request id")
	actor := flags.String("actor", "", "deprecated: only sent when provided and only works when server actor override is enabled")
	comment := flags.String("comment", "", "decision comment")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("%s requires a requirement id", command)
	}
	if strings.TrimSpace(*requestID) == "" {
		return fmt.Errorf("%s requires --request-id", command)
	}
	client, err := app.client()
	if err != nil {
		return err
	}
	input := api.ApprovalDecisionRequest{
		Actor:   optionalActor(flags, *actor),
		Comment: strings.TrimSpace(*comment),
	}
	var request api.ApprovalRequest
	if approve {
		request, err = client.ApproveRequirement(ctx, strings.TrimSpace(*requestID), flags.Arg(0), input)
	} else {
		request, err = client.RejectRequirement(ctx, strings.TrimSpace(*requestID), flags.Arg(0), input)
	}
	if err != nil {
		return governanceCommandError(command, flagProvided(flags, "actor"), err)
	}
	printApprovalRequest(app.output(), request)
	return nil
}

func printModuleOwners(w io.Writer, response api.ListModuleOwnersResponse) {
	if len(response.Owners) == 0 {
		fmt.Fprintln(w, "No owners or maintainers configured for this module.")
		return
	}
	fmt.Fprintln(w, "OWNER ID\tSUBJECT TYPE\tSUBJECT\tROLE")
	for _, owner := range response.Owners {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", owner.ID, owner.SubjectType, owner.Subject, owner.Role)
	}
}

func printApprovalRequest(w io.Writer, request api.ApprovalRequest) {
	fmt.Fprintf(w, "Approval request: %s\n", request.ID)
	fmt.Fprintf(w, "Status: %s\n", request.Status)
	if request.ModuleName != "" {
		fmt.Fprintf(w, "Module: %s\n", request.ModuleName)
	}
	if request.BreakingReportID != "" {
		fmt.Fprintf(w, "Breaking report: %s\n", request.BreakingReportID)
	}
	fmt.Fprintf(w, "Approvals: %d/%d\n", request.ReceivedApprovals, request.RequiredApprovals)
	if len(request.Requirements) == 0 {
		fmt.Fprintln(w, "Requirements: none")
	} else {
		fmt.Fprintln(w, "Requirements:")
		for _, requirement := range request.Requirements {
			target := requirement.TargetModuleName
			if target == "" {
				target = requirement.TargetModuleID
			}
			fmt.Fprintf(w, "- %s [%s] target=%s role=%s status=%s", requirement.ID, requirement.RequirementType, target, requirement.RequiredRole, requirement.Status)
			if strings.TrimSpace(requirement.Reason) != "" {
				fmt.Fprintf(w, " reason=%q", requirement.Reason)
			}
			fmt.Fprintln(w)
		}
	}
	if len(request.Decisions) > 0 {
		fmt.Fprintln(w, "Decisions:")
		for _, decision := range request.Decisions {
			fmt.Fprintf(w, "- %s requirement=%s decision=%s decided_by=%s", decision.ID, decision.RequirementID, decision.Decision, decision.DecidedBy)
			if strings.TrimSpace(decision.Comment) != "" {
				fmt.Fprintf(w, " comment=%q", decision.Comment)
			}
			fmt.Fprintln(w)
		}
	}
}

func optionalActor(flags *flag.FlagSet, value string) string {
	if !flagProvided(flags, "actor") {
		return ""
	}
	return strings.TrimSpace(value)
}

func flagProvided(flags *flag.FlagSet, name string) bool {
	provided := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			provided = true
		}
	})
	return provided
}

func governanceCommandError(command string, actorProvided bool, err error) error {
	if actorProvided && isGovernanceActorForbidden(err) {
		return fmt.Errorf("%s failed: server rejected the provided --actor; omit --actor so the server uses the authenticated principal, or enable server actor override for migration compatibility: %w", command, err)
	}
	return fmt.Errorf("%s failed: %w", command, err)
}

func isGovernanceActorForbidden(err error) bool {
	var apiErr api.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == 403 && strings.Contains(apiErr.Body, "Governance actor is not allowed")
}

func validSubjectType(value string) bool {
	switch strings.TrimSpace(value) {
	case "user", "team":
		return true
	default:
		return false
	}
}

func validOwnerRole(value string) bool {
	switch strings.TrimSpace(value) {
	case "owner", "maintainer":
		return true
	default:
		return false
	}
}
