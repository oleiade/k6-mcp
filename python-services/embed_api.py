from flask import Flask, request, jsonify
from sentence_transformers import SentenceTransformer

app = Flask(__name__)
model = SentenceTransformer('all-MiniLM-L6-v2')

@app.route('/embed', methods=['POST'])
def embed():
    data = request.get_json()
    if not data or 'text' not in data:
        return jsonify({"error": "Missing 'text' in request body"}), 400

    text = data['text']
    embedding = model.encode(text).tolist()
    return jsonify({"embedding": embedding})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5001)