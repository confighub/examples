#!/bin/bash
# Get instance information for demo purposes (no AWS CLI needed)
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)
HOSTNAME=$(hostname)

# Get instance name from metadata - with better error handling
INSTANCE_NAME=$(curl -s http://169.254.169.254/latest/meta-data/tags/instance/Name 2>/dev/null)
if [ -z "$INSTANCE_NAME" ] || [[ "$INSTANCE_NAME" == *"404"* ]] || [[ "$INSTANCE_NAME" == *"Not Found"* ]]; then
  INSTANCE_NAME="server1"
fi

# Create demo hello.txt file with instance information
echo "Hello from EC2 Instance!" > /home/ubuntu/hello.txt
echo "Instance ID: $INSTANCE_ID" >> /home/ubuntu/hello.txt
echo "Instance Name: $INSTANCE_NAME" >> /home/ubuntu/hello.txt
echo "Hostname: $HOSTNAME" >> /home/ubuntu/hello.txt
echo "Timestamp: $(date)" >> /home/ubuntu/hello.txt
echo "Memory:" >> /home/ubuntu/hello.txt
free -h >> /home/ubuntu/hello.txt

# Make the file readable by everyone
chmod 644 /home/ubuntu/hello.txt

# Add final status to hello.txt
echo "Instance setup completed successfully!" >> /home/ubuntu/hello.txt
