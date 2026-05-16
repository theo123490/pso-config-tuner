import math

import numpy as np
import plotly.graph_objects as go
from flask import Flask, request, jsonify

app = Flask(__name__)

# Scales the height of the outer exponential term; higher values deepen the global minimum basin,
# making it harder for particles to escape local optima near the origin.
amplitude = 20.0

# Controls how quickly the outer exponential decays with distance from the origin;
# lower values widen the basin of attraction, higher values create a sharper, narrower funnel.
decay = 0.4

# Angular frequency of the cosine modulation; determines how densely local minima are packed
# across the search space — 2π produces one full oscillation per unit distance.
frequency = 1.2 * math.pi


def ackley(x1, x2):
    sum_sq  = (x1 ** 2 + x2 ** 2) / 2.0
    sum_cos = (math.cos(frequency * x1) + math.cos(frequency * x2)) / 2.0
    return -amplitude * math.exp(-decay * math.sqrt(sum_sq)) - math.exp(sum_cos) + amplitude + math.e


@app.route("/fitness", methods=["POST"])
def fitness():
    data = request.get_json(force=True)
    metrics = data.get("metrics", {})

    x1 = float(metrics.get("x1", 0.0))
    x2 = float(metrics.get("x2", 0.0))

    # PSO maximizes; Ackley minimum is 0 at origin so negate → score 0 = optimal
    score = -ackley(x1, x2)

    return jsonify({"score": score})


@app.route("/render")
def render():
    x1_q = request.args.get("x1")
    x2_q = request.args.get("x2")

    grid = np.linspace(-10, 10, 200)
    X1, X2 = np.meshgrid(grid, grid)
    Z = np.vectorize(ackley)(X1, X2)

    fig = go.Figure()

    fig.add_trace(go.Surface(
        x=grid, y=grid, z=Z,
        colorscale="Viridis",
        opacity=0.85,
        colorbar=dict(title="Ackley value"),
        name="Ackley surface",
    ))

    fig.add_trace(go.Scatter3d(
        x=[0], y=[0], z=[0],
        mode="markers",
        marker=dict(size=6, color="red", symbol="diamond"),
        name="Global minimum (0, 0)",
    ))

    if x1_q is not None and x2_q is not None:
        try:
            px1, px2 = float(x1_q), float(x2_q)
            pz = ackley(px1, px2)
            fig.add_trace(go.Scatter3d(
                x=[px1], y=[px2], z=[pz],
                mode="markers+text",
                marker=dict(size=8, color="orange", symbol="circle"),
                text=[f"({px1:.4f}, {px2:.4f})"],
                textposition="top center",
                name="Query point",
            ))
        except ValueError:
            pass

    fig.update_layout(
        title=dict(text="Ackley Function", font=dict(size=20)),
        scene=dict(
            xaxis_title="x1",
            yaxis_title="x2",
            zaxis_title="Ackley(x1, x2)",
            camera=dict(eye=dict(x=1.6, y=1.6, z=0.8)),
        ),
        margin=dict(l=0, r=0, t=50, b=0),
        legend=dict(x=0, y=1),
    )

    return fig.to_html(full_html=True, include_plotlyjs="cdn")


@app.route("/health")
def health():
    return jsonify({"status": "ok"})


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=9000)
