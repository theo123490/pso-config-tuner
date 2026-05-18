from flask import Flask, request, jsonify

app = Flask(__name__)

# TODO: implement real fitness scoring against a live or simulated Kafka cluster.
# Metrics received from the controller will contain the particle's current config:
#   num_consumers, max_poll_records, fetch_min_bytes, session_timeout_ms
# The score should reflect consumer throughput and latency — higher = better.


@app.route("/fitness", methods=["POST"])
def fitness():
    data = request.get_json(force=True)
    metrics = data.get("metrics", {})

    # Placeholder: return 0 until real evaluation logic is wired in.
    _ = metrics
    score = 0.0

    return jsonify({"score": score})


@app.route("/health")
def health():
    return jsonify({"status": "ok"})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=9000)
