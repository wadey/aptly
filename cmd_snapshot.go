package main

import (
	"fmt"
	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
	"github.com/smira/aptly/debian"
	"github.com/wsxiaoys/terminal/color"
	"strings"
)

func aptlySnapshotCreate(cmd *commander.Command, args []string) error {
	var err error

	if len(args) < 4 || args[1] != "from" || args[2] != "mirror" {
		cmd.Usage()
		return err
	}

	repoName, mirrorName := args[3], args[0]

	repoCollection := debian.NewRemoteRepoCollection(context.database)
	repo, err := repoCollection.ByName(repoName)
	if err != nil {
		return fmt.Errorf("unable to create snapshot: %s", err)
	}

	err = repoCollection.LoadComplete(repo)
	if err != nil {
		return fmt.Errorf("unable to create snapshot: %s", err)
	}

	snapshot, err := debian.NewSnapshotFromRepository(mirrorName, repo)
	if err != nil {
		return fmt.Errorf("unable to create snapshot: %s", err)
	}

	snapshotCollection := debian.NewSnapshotCollection(context.database)

	err = snapshotCollection.Add(snapshot)
	if err != nil {
		return fmt.Errorf("unable to add snapshot: %s", err)
	}

	fmt.Printf("\nSnapshot %s successfully created.\nYou can run 'aptly publish snapshot %s' to publish snapshot as Debian repository.\n", snapshot.Name, snapshot.Name)

	return err
}

func aptlySnapshotList(cmd *commander.Command, args []string) error {
	var err error
	if len(args) != 0 {
		cmd.Usage()
		return err
	}

	snapshotCollection := debian.NewSnapshotCollection(context.database)

	if snapshotCollection.Len() > 0 {
		fmt.Printf("List of snapshots:\n")
		snapshotCollection.ForEach(func(snapshot *debian.Snapshot) error {
			fmt.Printf(" * %s\n", snapshot)
			return nil
		})

		fmt.Printf("\nTo get more information about snapshot, run `aptly snapshot show <name>`.\n")
	} else {
		fmt.Printf("\nNo snapshots found, create one with `aptly snapshot create...`.\n")
	}
	return err

}

func aptlySnapshotShow(cmd *commander.Command, args []string) error {
	var err error
	if len(args) != 1 {
		cmd.Usage()
		return err
	}

	name := args[0]

	snapshotCollection := debian.NewSnapshotCollection(context.database)
	snapshot, err := snapshotCollection.ByName(name)
	if err != nil {
		return fmt.Errorf("unable to show: %s", err)
	}

	err = snapshotCollection.LoadComplete(snapshot)
	if err != nil {
		return fmt.Errorf("unable to show: %s", err)
	}

	fmt.Printf("Name: %s\n", snapshot.Name)
	fmt.Printf("Created At: %s\n", snapshot.CreatedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Description: %s\n", snapshot.Description)
	fmt.Printf("Number of packages: %d\n", snapshot.NumPackages())
	fmt.Printf("Packages:\n")

	packageCollection := debian.NewPackageCollection(context.database)

	err = snapshot.RefList().ForEach(func(key []byte) error {
		p, err := packageCollection.ByKey(key)
		if err != nil {
			return err
		}
		fmt.Printf("  %s\n", p)
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to load packages: %s", err)
	}

	return err
}

func aptlySnapshotVerify(cmd *commander.Command, args []string) error {
	var err error
	if len(args) < 1 {
		cmd.Usage()
		return err
	}

	snapshotCollection := debian.NewSnapshotCollection(context.database)
	packageCollection := debian.NewPackageCollection(context.database)

	snapshots := make([]*debian.Snapshot, len(args))
	for i := range snapshots {
		snapshots[i], err = snapshotCollection.ByName(args[i])
		if err != nil {
			return fmt.Errorf("unable to verify: %s", err)
		}

		err = snapshotCollection.LoadComplete(snapshots[i])
		if err != nil {
			return fmt.Errorf("unable to verify: %s", err)
		}
	}

	packageList, err := debian.NewPackageListFromRefList(snapshots[0].RefList(), packageCollection)
	if err != nil {
		fmt.Errorf("unable to load packages: %s", err)
	}

	sourcePackageList := debian.NewPackageList()
	err = sourcePackageList.Append(packageList)
	if err != nil {
		fmt.Errorf("unable to merge sources: %s", err)
	}

	for i := 1; i < len(snapshots); i++ {
		pL, err := debian.NewPackageListFromRefList(snapshots[i].RefList(), packageCollection)
		if err != nil {
			fmt.Errorf("unable to load packages: %s", err)
		}

		err = sourcePackageList.Append(pL)
		if err != nil {
			fmt.Errorf("unable to merge sources: %s", err)
		}
	}

	sourcePackageList.PrepareIndex()

	var architecturesList []string

	architectures := cmd.Flag.Lookup("architectures").Value.String()
	if architectures != "" {
		architecturesList = strings.Split(architectures, ",")
	} else {
		architecturesList = packageList.Architectures()
	}

	if len(architecturesList) == 0 {
		return fmt.Errorf("unable to determine list of architectures, please specify explicitly")
	}

	missing, err := packageList.VerifyDependencies(0, architecturesList, sourcePackageList)
	if err != nil {
		return fmt.Errorf("unable to verify dependencies: %s", err)
	}

	if len(missing) == 0 {
		fmt.Printf("All dependencies are satisfied.\n")
	} else {
		fmt.Printf("Missing dependencies (%d):\n", len(missing))
		for _, dep := range missing {
			fmt.Printf("  %s\n", dep.String())
		}
	}

	return err
}

func aptlySnapshotPull(cmd *commander.Command, args []string) error {
	var err error
	if len(args) < 4 {
		cmd.Usage()
		return err
	}

	snapshotCollection := debian.NewSnapshotCollection(context.database)
	packageCollection := debian.NewPackageCollection(context.database)

	// Load <name> snapshot
	snapshot, err := snapshotCollection.ByName(args[0])
	if err != nil {
		return fmt.Errorf("unable to pull: %s", err)
	}

	err = snapshotCollection.LoadComplete(snapshot)
	if err != nil {
		return fmt.Errorf("unable to pull: %s", err)
	}

	// Load <source> snapshot
	source, err := snapshotCollection.ByName(args[1])
	if err != nil {
		return fmt.Errorf("unable to pull: %s", err)
	}

	err = snapshotCollection.LoadComplete(source)
	if err != nil {
		return fmt.Errorf("unable to pull: %s", err)
	}

	fmt.Printf("Dependencies would be pulled into snapshot:\n    %s\nfrom snapshot:\n    %s\nand result would be saved as new snapshot %s.\n",
		snapshot, source, args[2])

	// Convert snapshot to package list
	fmt.Printf("Loading packages (%d)...\n", snapshot.RefList().Len()+source.RefList().Len())
	packageList, err := debian.NewPackageListFromRefList(snapshot.RefList(), packageCollection)
	if err != nil {
		return fmt.Errorf("unable to load packages: %s", err)
	}

	sourcePackageList, err := debian.NewPackageListFromRefList(source.RefList(), packageCollection)
	if err != nil {
		return fmt.Errorf("unable to load packages: %s", err)
	}

	fmt.Printf("Building indexes...\n")
	packageList.PrepareIndex()
	sourcePackageList.PrepareIndex()

	// Calculate architectures
	var architecturesList []string

	architectures := cmd.Flag.Lookup("architectures").Value.String()
	if architectures != "" {
		architecturesList = strings.Split(architectures, ",")
	} else {
		architecturesList = packageList.Architectures()
	}

	if len(architecturesList) == 0 {
		return fmt.Errorf("unable to determine list of architectures, please specify explicitly")
	}

	// Initial dependencies out of arguments
	initialDependencies := make([]debian.Dependency, len(args)-3)
	for i, arg := range args[3:] {
		initialDependencies[i], err = debian.ParseDependency(arg)
		if err != nil {
			return fmt.Errorf("unable to parse argument: %s", err)
		}
	}

	// Perform pull
	for _, arch := range architecturesList {
		dependencies := make([]debian.Dependency, len(initialDependencies), 128)
		for i := range dependencies {
			dependencies[i] = initialDependencies[i]
			dependencies[i].Architecture = arch
		}

		// Go over list of initial dependencies + list of dependencies found
		for i := 0; i < len(dependencies); i++ {
			dep := dependencies[i]

			// Search for package that can satisfy dependencies
			pkg := sourcePackageList.Search(dep)
			if pkg == nil {
				color.Printf("@y[!]@| @!Dependency %s can't be satisfied with source %s@|\n", &dep, source)
				continue
			}

			// Remove all packages with the same name and architecture
			for p := packageList.Search(debian.Dependency{Architecture: arch, Pkg: pkg.Name}); p != nil; {
				packageList.Remove(p)
				color.Printf("@r[-]@| %s removed\n", p)
				p = packageList.Search(debian.Dependency{Architecture: arch, Pkg: pkg.Name})
			}

			// Add new discovered package
			packageList.Add(pkg)
			color.Printf("@g[+]@| %s added\n", pkg)

			// Find missing dependencies for single added package
			pL := debian.NewPackageList()
			pL.Add(pkg)

			missing, err := pL.VerifyDependencies(0, []string{arch}, packageList)
			if err != nil {
				color.Printf("@y[!]@| @!Error while verifying dependencies for pkg %s: %s@|\n", pkg, err)
			}

			// Append missing dependencies to the list of dependencies to satisfy
			for _, misDep := range missing {
				found := false
				for _, d := range dependencies {
					if d == misDep {
						found = true
						break
					}
				}

				if !found {
					dependencies = append(dependencies, misDep)
				}
			}
		}
	}

	// Create <destination> snapshot
	destination := debian.NewSnapshotFromPackageList(args[2], []*debian.Snapshot{snapshot, source}, packageList,
		fmt.Sprintf("Pulled into '%s' with '%s' as source, pull request was: '%s'", snapshot.Name, source.Name, strings.Join(args[3:], " ")))

	err = snapshotCollection.Add(destination)
	if err != nil {
		return fmt.Errorf("unable to create snapshot: %s", err)
	}

	fmt.Printf("\nSnapshot %s successfully created.\nYou can run 'aptly publish snapshot %s' to publish snapshot as Debian repository.\n", destination.Name, destination.Name)

	return err
}

func makeCmdSnapshotCreate() *commander.Command {
	cmd := &commander.Command{
		Run:       aptlySnapshotCreate,
		UsageLine: "create <name> from mirror <mirror-name>",
		Short:     "creates snapshot out of any mirror",
		Long: `
Create makes persistent immutable snapshot of repository mirror state in givent moment of time.
`,
		Flag: *flag.NewFlagSet("aptly-snapshot-create", flag.ExitOnError),
	}

	return cmd

}

func makeCmdSnapshotList() *commander.Command {
	cmd := &commander.Command{
		Run:       aptlySnapshotList,
		UsageLine: "list",
		Short:     "lists snapshots",
		Long: `
list shows full list of snapshots created.

ex:
  $ aptly snapshot list
`,
		Flag: *flag.NewFlagSet("aptly-snapshot-list", flag.ExitOnError),
	}

	return cmd
}

func makeCmdSnapshotShow() *commander.Command {
	cmd := &commander.Command{
		Run:       aptlySnapshotShow,
		UsageLine: "show <name>",
		Short:     "shows details about snapshot",
		Long: `
Show shows full information about snapshot.
`,
		Flag: *flag.NewFlagSet("aptly-snapshot-show", flag.ExitOnError),
	}

	return cmd
}

func makeCmdSnapshotVerify() *commander.Command {
	cmd := &commander.Command{
		Run:       aptlySnapshotVerify,
		UsageLine: "verify <name> [<source> ...]",
		Short:     "verifies that dependencies are satisfied in snapshot",
		Long: `
Verify does depenency resolution in snapshot, possibly using additional snapshots as dependency sources.
All unsatisfied dependencies are returned.
`,
		Flag: *flag.NewFlagSet("aptly-snapshot-verify", flag.ExitOnError),
	}

	cmd.Flag.String("architectures", "", "list of architectures to verify (comma-separated)")

	return cmd
}

func makeCmdSnapshotPull() *commander.Command {
	cmd := &commander.Command{
		Run:       aptlySnapshotPull,
		UsageLine: "pull <name> <source> <destination> <package-name> ...",
		Short:     "performs partial upgrades (pulls new packages) from another snapshot",
		Long: `
Pulls (upgrades) new packages (upgrades version) along with its dependencies in <name> snapshot
from <source> snapshot. New snapshot <destination> is created.
`,
		Flag: *flag.NewFlagSet("aptly-snapshot-pull", flag.ExitOnError),
	}

	cmd.Flag.String("architectures", "", "list of architectures to consider during pull (comma-separated)")

	return cmd
}

func makeCmdSnapshot() *commander.Command {
	return &commander.Command{
		UsageLine: "snapshot",
		Short:     "manage snapshots of repositories",
		Subcommands: []*commander.Command{
			makeCmdSnapshotCreate(),
			makeCmdSnapshotList(),
			makeCmdSnapshotShow(),
			makeCmdSnapshotVerify(),
			makeCmdSnapshotPull(),
			//makeCmdSnapshotDestroy(),
		},
		Flag: *flag.NewFlagSet("aptly-snapshot", flag.ExitOnError),
	}
}