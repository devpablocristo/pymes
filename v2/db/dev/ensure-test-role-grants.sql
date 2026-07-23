GRANT CONNECT, CREATE, TEMPORARY ON DATABASE pymes_v2_test TO pymes_migrator;
GRANT CONNECT, TEMPORARY ON DATABASE pymes_v2_test TO pymes_backend;
GRANT CONNECT, TEMPORARY ON DATABASE pymes_v2_test TO pymes_iam_worker;
GRANT USAGE, CREATE ON SCHEMA public TO pymes_migrator;
