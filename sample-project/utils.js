// Utility functions with various code quality issues

// Function with too many parameters
function processUserData(name, email, age, address, phone, country, city, zipcode, preferences) {
    // Missing input validation
    return {
        fullName: name,
        contact: email,
        userAge: age,
        location: address
    };
}

// Function with nested conditions (high complexity)
function validateUser(user) {
    if (user) {
        if (user.email) {
            if (user.email.includes('@')) {
                if (user.age) {
                    if (user.age > 18) {
                        if (user.name) {
                            if (user.name.length > 2) {
                                return true;
                            }
                        }
                    }
                }
            }
        }
    }
    return false;
}

// Magic numbers without constants
function calculateDiscount(price, userType) {
    if (userType === 1) {  // Magic number
        return price * 0.1;
    } else if (userType === 2) {  // Magic number
        return price * 0.15;
    } else if (userType === 3) {  // Magic number
        return price * 0.2;
    }
    return 0;
}

// Console.log statements left in code
function debugFunction(data) {
    console.log("Processing data:", data);  // Should be removed in production

    const result = data.map(item => {
        console.log("Processing item:", item);  // Debug statement
        return item * 2;
    });

    console.log("Result:", result);  // Another debug statement
    return result;
}

// Export without proper documentation
module.exports = {
    processUserData,
    validateUser,
    calculateDiscount,
    debugFunction
};