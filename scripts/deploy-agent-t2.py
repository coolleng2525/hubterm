#!/usr/bin/env python3
"""Deploy and manage hubterm-agent on lab hosts via t2-unified-toolkit."""

import argparse
import os
import shlex
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
HUBTERM_ROOT = os.path.dirname(SCRIPT_DIR)
T2_ROOT = os.path.join(HUBTERM_ROOT, ".cursor", "skills", "t2-unified-toolkit")
DEFAULT_TARGETS = ["10.223.40.20", "10.223.40.21"]
DEFAULT_BIN = os.path.join(HUBTERM_ROOT, "dist", "hubterm-agent-linux-amd64")
DEFAULT_DATA = "$HOME/data/hubterm-agent"

sys.path.insert(0, T2_ROOT)

from core import t2_config  # noqa: E402


def upload_file_via_scp(jargs, local_path, remote_path):
    """SCP upload via t2 jump profile; skip -J when jump host equals target."""
    import pexpect

    print(f"[*] Uploading {local_path} to {remote_path} via SCP...")
    scp_opts = [
        "-o",
        "StrictHostKeyChecking=no",
        "-o",
        "UserKnownHostsFile=/dev/null",
        "-o",
        "HostKeyAlgorithms=+ssh-rsa",
        "-o",
        "PubkeyAcceptedKeyTypes=+ssh-rsa",
    ]
    use_jump = bool(jargs.jump_host) and jargs.jump_host != jargs.target_host
    if use_jump:
        jump_str = (
            f"{jargs.jump_user}@{jargs.jump_host}"
            if jargs.jump_user
            else jargs.jump_host
        )
        scp_opts.extend(["-J", jump_str])

    scp_cmd = (
        f"scp {' '.join(scp_opts)} {local_path} "
        f"{jargs.target_user}@{jargs.target_host}:{remote_path}"
    )
    child = pexpect.spawn(scp_cmd, encoding="utf-8", dimensions=(24, 120))

    expect_list = []
    jump_pass_idx = -1
    if use_jump:
        expect_list.append(f"{jargs.jump_user}@{jargs.jump_host}'s password:")
        jump_pass_idx = len(expect_list) - 1

    target_pass_idx1 = len(expect_list)
    expect_list.append(f"{jargs.target_user}@{jargs.target_host}'s password:")
    target_pass_idx2 = len(expect_list)
    expect_list.append(r"Password:")
    eof_idx = len(expect_list)
    expect_list.append(pexpect.EOF)
    timeout_idx = len(expect_list)
    expect_list.append(pexpect.TIMEOUT)

    while True:
        index = child.expect(expect_list, timeout=120)
        if index == jump_pass_idx:
            if jargs.jump_pass:
                child.sendline(jargs.jump_pass)
            else:
                raise RuntimeError("Jump host password required for SCP")
        elif index in (target_pass_idx1, target_pass_idx2):
            if jargs.target_pass:
                child.sendline(jargs.target_pass)
            else:
                raise RuntimeError("Target password required for SCP")
        elif index == eof_idx:
            break
        elif index == timeout_idx:
            raise RuntimeError("SCP connection timed out")

    print("[+] Upload complete.")


def ssh_base_opts(jargs) -> list[str]:
    opts = [
        "ssh",
        "-o",
        "StrictHostKeyChecking=no",
        "-o",
        "UserKnownHostsFile=/dev/null",
        "-o",
        "HostKeyAlgorithms=+ssh-rsa",
        "-o",
        "PubkeyAcceptedKeyTypes=+ssh-rsa",
    ]
    use_jump = bool(jargs.jump_host) and jargs.jump_host != jargs.target_host
    if use_jump:
        jump_str = (
            f"{jargs.jump_user}@{jargs.jump_host}"
            if jargs.jump_user
            else jargs.jump_host
        )
        opts.extend(["-J", jump_str])
    opts.append(f"{jargs.target_user}@{jargs.target_host}")
    return opts


def run_remote(jargs, remote_script: str) -> str:
    cmd = ssh_base_opts(jargs) + ["bash", "-s"]
    result = subprocess.run(
        cmd,
        input=remote_script,
        capture_output=True,
        text=True,
        check=False,
    )
    output = (result.stdout or "").strip()
    err = (result.stderr or "").strip()
    if result.returncode != 0:
        detail = err or output or f"exit {result.returncode}"
        raise RuntimeError(detail)
    if err:
        print(err)
    return output


def yaml_value(key: str, path: str) -> str:
    with open(path, encoding="utf-8") as f:
        for line in f:
            stripped = line.strip()
            if stripped.startswith(f"{key}:"):
                val = stripped.split(":", 1)[1].strip().strip('"').strip("'")
                return val
    return ""


def hubterm_center_url() -> str:
    env_url = os.environ.get("HUBTERM_CENTER_URL", "").strip()
    if env_url:
        return env_url
    cfg = os.path.join(HUBTERM_ROOT, "config.yaml")
    if os.path.isfile(cfg):
        port = yaml_value("port", cfg) or "8080"
        center_host = (
            t2_config.load().get("env", {}).get("upload_server_ip") or "127.0.0.1"
        )
        return f"http://{center_host}:{port}"
    return "http://127.0.0.1:8080"


def build_binary(out_path: str, force: bool) -> None:
    if os.path.isfile(out_path) and os.access(out_path, os.X_OK) and not force:
        print(f"[*] Using existing binary: {out_path}")
        return
    print(f"[*] Building linux/amd64 hubterm-agent -> {out_path}")
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    subprocess.run(
        ["go", "build", "-o", out_path, "./cmd/agent"],
        cwd=HUBTERM_ROOT,
        check=True,
        env={**os.environ, "CGO_ENABLED": "0", "GOOS": "linux", "GOARCH": "amd64"},
    )
    os.chmod(out_path, 0o755)


def resolve_remote_bin(target_user: str, remote_path: str | None) -> str:
    if remote_path:
        return remote_path.replace("~/", "$HOME/")
    if target_user == "root":
        return "/usr/local/bin/hubterm-agent"
    return "$HOME/bin/hubterm-agent"


def load_jargs(profile: str):
    defaults, _, _, selected, _ = t2_config.get_jump_profile(profile)

    class JumpArgs:
        pass

    jargs = JumpArgs()
    for key, value in defaults.items():
        setattr(jargs, key, value)
    return jargs, selected


def prepare_jargs(profile: str):
    jargs, selected = load_jargs(profile)
    if (
        jargs.target_user == "root"
        and not jargs.target_pass
        and jargs.jump_user
        and jargs.jump_host == jargs.target_host
    ):
        jargs.target_user = jargs.jump_user
    return jargs, selected


def shell_path(path: str) -> str:
    if path.startswith("$HOME/"):
        return path
    return shlex.quote(path)


def remote_start_script(
    bin_path: str,
    data_dir: str,
    center_url: str,
    node_name: str | None,
    node_ip: str | None,
) -> str:
    name_expr = shlex.quote(node_name) if node_name else "$(hostname)"
    ip_arg = f" --ip {shlex.quote(node_ip)}" if node_ip else ""
    return f"""set -e
BIN={shell_path(bin_path)}
DATA={shell_path(data_dir)}
PIDFILE="$DATA/agent.pid"
LOG="$DATA/agent.log"
CENTER={shlex.quote(center_url)}
mkdir -p "$DATA" "$(dirname "$BIN")"
if [ ! -x "$BIN" ]; then
  echo "binary not found: $BIN" >&2
  exit 1
fi
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "already running pid=$(cat "$PIDFILE")"
  exit 0
fi
nohup "$BIN" --center "$CENTER" --data "$DATA" --name {name_expr}{ip_arg} >>"$LOG" 2>&1 &
echo $! > "$PIDFILE"
sleep 0.5
if kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "started pid=$(cat "$PIDFILE") center=$CENTER"
else
  echo "failed to start; see $LOG" >&2
  tail -n 20 "$LOG" 2>/dev/null || true
  exit 1
fi
"""


def remote_stop_script(data_dir: str) -> str:
    return f"""set -e
DATA={shell_path(data_dir)}
PIDFILE="$DATA/agent.pid"
if [ -f "$PIDFILE" ]; then
  PID=$(cat "$PIDFILE")
  if kill -0 "$PID" 2>/dev/null; then
    kill "$PID"
    echo "stopped pid=$PID"
  else
    echo "stale pid=$PID"
  fi
  rm -f "$PIDFILE"
else
  pkill -f '[h]ubterm-agent' && echo "stopped hubterm-agent" || echo "not running"
fi
"""


def remote_status_script(data_dir: str, bin_path: str) -> str:
    return f"""set +e
DATA={shell_path(data_dir)}
PIDFILE="$DATA/agent.pid"
BIN={shell_path(bin_path)}
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "running pid=$(cat "$PIDFILE") bin=$BIN"
  ps -p "$(cat "$PIDFILE")" -o pid,cmd --no-headers 2>/dev/null
  exit 0
fi
if pgrep -af '[h]ubterm-agent' >/dev/null 2>&1; then
  echo "running (no pidfile)"
  pgrep -af '[h]ubterm-agent'
  exit 0
fi
echo "stopped"
exit 1
"""


def deploy_to_target(
    profile: str, local_bin: str, remote_bin: str | None, data_dir: str
) -> tuple:
    jargs, selected = prepare_jargs(profile)
    remote = resolve_remote_bin(jargs.target_user, remote_bin)
    print(
        f"\n[*] Deploy {os.path.basename(local_bin)} -> "
        f"{selected} ({jargs.target_user}@{jargs.target_host}:{remote})"
    )
    run_remote(jargs, "mkdir -p ~/bin 2>/dev/null || mkdir -p /usr/local/bin")
    upload_file_via_scp(jargs, local_bin, remote.replace("$HOME/", "~/"))
    run_remote(jargs, f"chmod +x {shell_path(remote)}")
    print(f"[+] Deployed to {selected}")
    return jargs, selected, remote


def resolve_node_identity(profile: str, node_name: str | None, node_ip: str | None):
    jargs, selected = prepare_jargs(profile)
    ip = node_ip or jargs.target_host
    name = node_name or ip or selected
    return jargs, selected, ip, name


def start_on_target(
    profile: str,
    center_url: str,
    remote_bin: str | None,
    data_dir: str,
    node_name: str | None,
    node_ip: str | None = None,
) -> None:
    jargs, selected, ip, name = resolve_node_identity(profile, node_name, node_ip)
    remote = resolve_remote_bin(jargs.target_user, remote_bin)
    print(f"\n[*] Start agent on {selected} ({jargs.target_host}) ip={ip} name={name}")
    out = run_remote(
        jargs, remote_start_script(remote, data_dir, center_url, name, ip)
    )
    print(out)


def stop_on_target(profile: str, data_dir: str) -> None:
    jargs, selected = prepare_jargs(profile)
    print(f"\n[*] Stop agent on {selected} ({jargs.target_host})")
    out = run_remote(jargs, remote_stop_script(data_dir))
    print(out)


def status_on_target(profile: str, data_dir: str, remote_bin: str | None) -> bool:
    jargs, selected = prepare_jargs(profile)
    remote = resolve_remote_bin(jargs.target_user, remote_bin)
    print(f"\n[*] Status {selected} ({jargs.target_host})")
    try:
        out = run_remote(jargs, remote_status_script(data_dir, remote))
        print(out)
        return True
    except RuntimeError as exc:
        print(str(exc))
        return False


def run_for_targets(action, targets, **kwargs) -> int:
    failed = []
    for target in targets:
        try:
            action(target, **kwargs)
        except Exception as exc:
            print(f"[-] Failed on {target}: {exc}", file=sys.stderr)
            failed.append(target)
    if failed:
        print(f"\n[-] Failed targets: {', '.join(failed)}", file=sys.stderr)
        return 1
    return 0


def cmd_deploy(args) -> int:
    build_binary(args.output, args.build)
    center_url = hubterm_center_url()
    if args.show_center:
        print(f"[*] Center URL: {center_url}")

    rc = run_for_targets(
        lambda t, **_: deploy_to_target(t, args.output, args.remote_path, args.data_dir),
        args.targets,
    )
    if rc != 0:
        return rc

    if args.start:
        rc = run_for_targets(
            lambda t, **_: start_on_target(
                t, center_url, args.remote_path, args.data_dir, args.name, args.ip
            ),
            args.targets,
        )
        if rc != 0:
            return rc

    print("\n[+] Deploy completed.")
    if not args.start:
        print(f"[*] Start: {sys.argv[0]} start {' '.join(args.targets)}")
    return 0


def cmd_start(args) -> int:
    center_url = args.center or hubterm_center_url()
    print(f"[*] Center URL: {center_url}")
    rc = run_for_targets(
        lambda t, **_: start_on_target(
            t, center_url, args.remote_path, args.data_dir, args.name, args.ip
        ),
        args.targets,
    )
    return rc


def cmd_stop(args) -> int:
    return run_for_targets(
        lambda t, **_: stop_on_target(t, args.data_dir),
        args.targets,
    )


def cmd_restart(args) -> int:
    center_url = args.center or hubterm_center_url()
    print(f"[*] Center URL: {center_url}")

    def restart_one(target: str) -> None:
        stop_on_target(target, args.data_dir)
        start_on_target(
            target,
            center_url,
            args.remote_path,
            args.data_dir,
            args.name,
            args.ip,
        )

    return run_for_targets(restart_one, args.targets)


def cmd_status(args) -> int:
    ok = True
    for target in args.targets:
        if not status_on_target(target, args.data_dir, args.remote_path):
            ok = False
    return 0 if ok else 1


def add_common_args(parser, include_build=False):
    parser.add_argument(
        "targets",
        nargs="*",
        default=DEFAULT_TARGETS,
        help="t2 jump profile names or IPs",
    )
    parser.add_argument(
        "-r",
        "--remote-path",
        default=None,
        help="Remote binary path (default: ~/bin/hubterm-agent)",
    )
    parser.add_argument(
        "--data-dir",
        default=DEFAULT_DATA,
        help=f"Remote data directory (default: {DEFAULT_DATA})",
    )
    parser.add_argument("--name", default=None, help="Node display name (default: target IP)")
    parser.add_argument("--ip", default=None, help="Reported node IP (default: target_host from t2 profile)")
    if include_build:
        parser.add_argument(
            "-o",
            "--output",
            default=DEFAULT_BIN,
            help="Local binary output path",
        )
        parser.add_argument("--build", action="store_true", help="Force rebuild binary")
        parser.add_argument(
            "--show-center",
            action="store_true",
            help="Print center URL",
        )
        parser.add_argument(
            "--start",
            action="store_true",
            help="Start agent after deploy",
        )


def main() -> None:
    if not os.path.isdir(T2_ROOT):
        print(f"[-] t2-unified-toolkit not found: {T2_ROOT}", file=sys.stderr)
        sys.exit(1)

    parser = argparse.ArgumentParser(
        description="Deploy and manage hubterm-agent on lab hosts (t2 jump profiles)."
    )
    sub = parser.add_subparsers(dest="command", required=True)

    p_deploy = sub.add_parser("deploy", help="Build and upload binary")
    add_common_args(p_deploy, include_build=True)
    p_deploy.set_defaults(func=cmd_deploy)

    p_start = sub.add_parser("start", help="Start remote agent (background)")
    add_common_args(p_start)
    p_start.add_argument("--center", default=None, help="Center URL override")
    p_start.set_defaults(func=cmd_start)

    p_stop = sub.add_parser("stop", help="Stop remote agent")
    add_common_args(p_stop)
    p_stop.set_defaults(func=cmd_stop)

    p_restart = sub.add_parser("restart", help="Restart remote agent")
    add_common_args(p_restart)
    p_restart.add_argument("--center", default=None, help="Center URL override")
    p_restart.set_defaults(func=cmd_restart)

    p_status = sub.add_parser("status", help="Show remote agent status")
    add_common_args(p_status)
    p_status.set_defaults(func=cmd_status)

    args = parser.parse_args()
    sys.exit(args.func(args))


if __name__ == "__main__":
    main()
