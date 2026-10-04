# Export snapshots to Git

## Prerequisites

A running operator, a writable Git repository, and a credentials Secret. Start with
[Your first inventory](../getting-started/first-inventory.md) for the full credential flow.

## Apply

Use the validated minimal sample, then set its endpoint and Secret reference for your repository:

```sh
kubectl apply -f config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_git_minimal.yaml
```

## Verify

```sh
kubectl wait --for=condition=ConnectionVerified ksnap/git-inventory-minimal -n default --timeout=90s
kubectl get ksnap/git-inventory-minimal -n default -o yaml
```

## Safe pruning of resource trees

Git and GitLab tree exports record their complete file list under the reserved
`.kollect-prune/` directory, in the same commit as the exported files. A subsequent
export removes only paths previously recorded for that inventory. This removes a
kind directory's last resource file without sweeping neighboring inventories or
custom directory layouts. Empty directories are not removed.

Ownership uses the cluster, inventory namespace, and inventory name, and stays
stable across generations and multipart suffixes. Distinct inventories can share a
repository when their file paths do not overlap. An export that claims a path
recorded for another inventory fails before modifying files. Use separate branches
or repositories for multiple sinks exporting the same inventory with different
layouts: they otherwise share one ownership record.

On upgrade, a missing record preserves unknown historical files and adopts only
the current export's files. Remove pre-upgrade orphan files manually after checking
their provenance; directory depth is never treated as proof of ownership. Renaming
an inventory or changing its cluster identity starts a new owner and likewise
preserves the previous owner's files. Keep an explicit `layout.mode: perResource`
or `split` when an empty inventory must retain its tree layout and prune its last
files; content-based auto-detection cannot infer a resource layout from no rows.

Multipart exports update ownership only with the final part's complete union.
Incomplete exports do not advance the record; files from an interrupted, never
completed export may require manual cleanup. The ownership record is separate from
the multipart completeness manifest and does not imply that a partial set is complete.

Treat ownership metadata as part of the repository's trusted state. Do not edit or
remove records to make a failing export pass. Malformed records, traversal, Git
internal paths, symlink components, and conflicting claims fail closed. Metadata
is limited to 16 MiB per repository branch, 1,024 owners, 100,000 paths per owner,
and 4,096 bytes per path or owner identifier. The `.kollect-prune/` path is reserved
and cannot be selected as an export destination.

## If it didn't work

Describe the sink and check repository permissions, TLS trust, branch, and Secret keys.

## Cleanup

```sh
kubectl delete -f config/samples/advanced/kollect_v1alpha1_kollectsnapshotsink_git_minimal.yaml
```

## Further reading

[Snapshot sink reference](../crds/kollectsnapshotsink.md) ·
[Git layout ADR](../adr/0419-git-export-serialization-layout.md)

Cluster inventories and namespaced inventories in namespace `cluster` share a
legacy export identity. Their exports retain stale files rather than pruning an
ambiguous owner; remove obsolete files manually. Other namespaces use recorded
ownership pruning as described above.
