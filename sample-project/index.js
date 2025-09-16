// Sample Node.js application with intentional code issues
const express = require('express');

// Missing semicolon and var usage
var app = express()
var port = 3000

// Assignment instead of comparison
function checkUser(user) {
    if (user.id = 1) {  // Should be === or ==
        console.log("Admin user");
    }

    // Unused variable
    let unusedVar = "this will never be used";

    // Missing return statement
    if (user.role === 'admin') {
        user.permissions = ['read', 'write', 'delete']
    }
}

// Function with no error handling
app.get('/api/users/:id', (req, res) => {
    const userId = req.params.id;

    // No input validation
    const user = getUserById(userId);
    res.json(user);
});

// Function that could throw error
function getUserById(id) {
    const users = [
        { id: 1, name: 'John', role: 'admin' },
        { id: 2, name: 'Jane', role: 'user' }
    ];

    return users.find(user => user.id == id);  // Should use strict equality
}

// Missing error handling in server start
app.listen(port, () => {
    console.log(`Server running on port ${port}`)  // Missing semicolon
});// Added a new function with issues
function testFunction() {
    var x = 1
    if (x = 2) {  // Assignment instead of comparison
        console.log('This has issues')
    }
}
