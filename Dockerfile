# Using the official MongoDB 5.0 image
FROM mongo:5.0

# Set environment variables for the root user
ENV MONGO_INITDB_ROOT_USERNAME=username
ENV MONGO_INITDB_ROOT_PASSWORD=password

# Create a mount point for the data
VOLUME /data/db

# Open the MongoDB port
EXPOSE 27017

# Basic command with memory optimization options
CMD ["mongod",\
     "--bind_ip_all", \
     "--wiredTigerCacheSizeGB=0.25", \
     "--nojournal"]