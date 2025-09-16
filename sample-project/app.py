#!/usr/bin/env python3

# Python sample with common code quality issues

import os
import sys
import requests
from flask import Flask,request,jsonify  # Poor import formatting

app=Flask(__name__)  # Missing spaces around operator

# Global variable (not recommended)
users_data=[]

def get_user_by_id(user_id):
    # No type hints
    # No docstring
    for user in users_data:
        if user['id']==user_id:  # Missing spaces around operator
            return user
    return None

@app.route('/users/<user_id>')
def get_user(user_id):
    # No input validation
    user=get_user_by_id(int(user_id))  # Could raise ValueError

    if user:
        return jsonify(user)
    else:
        return jsonify({'error':'User not found'}),404  # Missing spaces

def process_user_data(name,email,age):  # Missing spaces after commas
    # Function too long, doing too many things
    if not name:
        raise ValueError("Name is required")
    if not email:
        raise ValueError("Email is required")
    if age<0:  # Missing spaces
        raise ValueError("Age must be positive")
    if age>150:  # Missing spaces
        raise ValueError("Age too high")

    # Hardcoded values
    if age<18:
        category="minor"
    elif age<65:
        category="adult"
    else:
        category="senior"

    # Security issue - using eval
    risk_score=eval(f"{age}*0.1")  # Never use eval!

    return {
        'name':name,
        'email':email,
        'age':age,
        'category':category,
        'risk_score':risk_score
    }

# Missing main guard
app.run(debug=True,host='0.0.0.0')  # Debug mode in production, security risk