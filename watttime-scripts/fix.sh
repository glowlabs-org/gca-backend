#!/bin/bash

# This script performs in-place modifications on all Python (.py) files within a
# specified folder. It carries out several tasks:
#   1. Opens every Python file in a folder.
#   2. Adds code to load 'username' and 'password' from respective files.
#   3. Replaces all instances of the string 'glowlabs' with the loaded username.
#   4. Replaces all instances of the string 'ihavemyfuncreatinghardpasswords' with the loaded password.

# Loop through all the .py files in the current folder
for file in *.py; do
    # Read the contents of the file into a variable
    content=$(<"$file")

    # Identify the first double newline in the content
    double_newline=$(echo -n "$content" | awk 'BEGIN { RS = "\n\n"; } { print NR; exit; }')

    # Divide the content into two parts at the first double newline
    part1=$(echo -n "$content" | awk "BEGIN { RS = \"\n\n\"; } NR < $double_newline { printf \"%s\n\n\", \$0; }")
    part2=$(echo -n "$content" | awk "BEGIN { RS = \"\n\n\"; } NR >= $double_newline { printf \"%s\n\n\", \$0; }")

    # Insert the lines to load the username and password variables after the first double newline
    new_content="${part1}# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

${part2}"

    # Replace occurrences of 'glowlabs' with the username variable and 'ihavemyfuncreatinghardpasswords' with the password variable
    new_content=$(echo -n "$new_content" | sed "s/glowlabs/\${username}/g")
    new_content=$(echo -n "$new_content" | sed "s/boxelderbugwinsworldseriesonmarsA1!/\${password}/g")

    # Write the modified content back to the file
    echo -n "$new_content" > "$file"
done

