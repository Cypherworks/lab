# Anti-affinity instance placement for the Incus cluster. Instances whose names
# share a base (the name without a trailing "-<n>") are kept on separate hosts,
# so an HA trio like etcd-1/etcd-2/etcd-3 never lands two members on one node.
# Among the conflict-free hosts the emptiest (most free memory) wins.
#
# Fail-safe by construction: any uncertainty (no conflict-free host, unreadable
# resources) returns without set_target(), so Incus falls back to its built-in
# placement. A scriptlet error would block ALL instance creation cluster-wide,
# so every field access defaults rather than raising.

def _group_of(name):
    # Base name without a trailing "-<digits>": etcd-1 -> etcd. A name without
    # that suffix is its own group and never conflicts with another.
    idx = name.rfind("-")
    if idx <= 0:
        return name
    suffix = name[idx + 1:]
    if suffix and suffix.isdigit():
        return name[:idx]
    return name

def _free_memory(member_name):
    # Bytes free on a member, or -1 if it can't be read (treated as least
    # preferred, never as an error).
    res = get_cluster_member_resources(member_name)
    if res == None:
        return -1
    mem = getattr(res, "memory", None)
    if mem == None:
        return -1
    return getattr(mem, "total", 0) - getattr(mem, "used", 0)

def instance_placement(request, candidate_members):
    name = request.name
    group = _group_of(name)

    # Disqualify any member already hosting a same-group instance. On a move or
    # evacuation the instance may already be counted somewhere, so skip our own
    # name.
    free = []
    for member in candidate_members:
        conflict = False
        for inst in get_instances(member.server_name, request.project):
            other = getattr(inst, "name", "")
            if other == name:
                continue
            if _group_of(other) == group:
                conflict = True
                break
        if not conflict:
            free.append(member)

    if len(free) == 0:
        log_warn("placement: no anti-affinity-free host for " + name +
                 " (group " + group + "); using default placement")
        return

    # Emptiest conflict-free host wins.
    best = free[0]
    best_free = _free_memory(best.server_name)
    for member in free[1:]:
        avail = _free_memory(member.server_name)
        if avail > best_free:
            best = member
            best_free = avail

    log_info("placement: " + name + " (group " + group + ") -> " +
             best.server_name)
    set_target(best.server_name)
