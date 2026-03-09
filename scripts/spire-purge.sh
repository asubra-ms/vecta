# Stop the management layer
sudo systemctl stop k3s
sudo k3s-killall.sh

# The Nuclear Option: Remove the underlying storage directories
# This is where the old 'example.org' SQLite DB lives
sudo rm -rf /var/lib/rancher/k3s/server/db
sudo rm -rf /run/spire
sudo rm -rf /var/lib/spire

# Clear any hidden mounts that might be 'protecting' old configs
sudo umount -l /var/lib/rancher/k3s/projected 2>/dev/null || true

