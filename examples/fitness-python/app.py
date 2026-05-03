"""
Example Python Fitness Calculator.

Implements POST /fitness — receives particle metrics from the Controller,
returns a scalar fitness score (higher = better).

Install: pip install flask
Run:     python app.py
"""

from flask import Flask, request, jsonify

app = Flask(__name__)


@app.route("/fitness", methods=["POST"])
def fitness():
    data = request.get_json(force=True)
    metrics = data.get("metrics", {})

    # Example scoring: maximize throughput, penalize latency and errors.
    # Replace with your domain-specific logic.
    throughput = metrics.get("throughput_rps", 0)
    latency = metrics.get("p99_latency_ms", 9999)
    error_rate = metrics.get("error_rate", 1.0)

    score = (throughput / 10000) - (latency / 5000) - (error_rate * 10)

    return jsonify({"score": score})


@app.route("/health")
def health():
    return jsonify({"status": "ok"})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=9000)
