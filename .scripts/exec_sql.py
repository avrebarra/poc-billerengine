import argparse
import sqlite3

def migrate_schema(schema_file, sqlite_url):
    # Read schema from schema file
    with open(schema_file, 'r') as file:
        schema = file.read()

    # Connect to SQLite database
    conn = sqlite3.connect(sqlite_url)
    cursor = conn.cursor()

    # Execute schema migration
    cursor.executescript(schema)

    # Commit changes and close connection
    conn.commit()
    conn.close()

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Python script to migrate file to SQLite database.')
    parser.add_argument('--file', type=str, help='Path to file', required=True)
    parser.add_argument('--sqliteurl', type=str, help='SQLite database URL', required=True)
    args = parser.parse_args()

    if not args.file or not args.sqliteurl:
        print("Please provide both schema file and SQLite database URL.")
    else:
        # Perform migration
        migrate_schema(args.file, args.sqliteurl)
