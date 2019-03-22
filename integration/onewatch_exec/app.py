from flask import Flask
import secrets
app = Flask(__name__)

@app.route('/')
def make_hypothesis():
  return "🍄 One-Up! 🍄"

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')
