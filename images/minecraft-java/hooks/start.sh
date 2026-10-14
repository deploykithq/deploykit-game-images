#!/usr/bin/env bash
. /opt/deploykit/lib/common.sh
. "$DK_STATE_DIR/launch.env"
cd "$DK_DATA_DIR" || exit 1

heap=${DK_HEAP_MB:?}
jvm=(-Xms"${heap}M" -Xmx"${heap}M" -XX:+UseG1GC -XX:+ParallelRefProcEnabled -XX:+DisableExplicitGC)
# Extra JVM flags, whitespace-separated (not a shell string: no expansion).
read -r -a extra <<<"${DK_VAR_JVM_OPTS:-}"

case $MC_LAUNCH_KIND in
    jar) exec java "${jvm[@]}" "${extra[@]}" -jar "$MC_LAUNCH_TARGET" nogui ;;
    args) exec java "${jvm[@]}" "${extra[@]}" @"$MC_LAUNCH_TARGET" nogui ;;
esac
dk_die "unknown launch kind '$MC_LAUNCH_KIND'"
