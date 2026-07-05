echo "=== Disk Usage (Human Readable) ==="
df -h
echo ""
echo "=== Inode Usage ==="
df -i
echo ""
echo "=== Disk Mounts ==="
mount | grep -E '^/dev' || true
echo "=== new for test ==="

