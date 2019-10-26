console.log("hello world")

const data = {
    "nodes": [
        {
            "id": "n0",
            "label": "A node",
        },
        {
            "id": "n1",
            "label": "Another node",
        },
        {
            "id": "n2",
            "label": "And a last one",
        }
    ],
    "edges": [
        {
            "id": "e0",
            "source": "n0",
            "target": "n1"
        },
        {
            "id": "e1",
            "source": "n1",
            "target": "n2"
        },
        {
            "id": "e2",
            "source": "n2",
            "target": "n0"
        }
    ]
}

const s = new sigma('graphContainer')
s.graph.addNode({
    // Main attributes:
    id: 'n0',
    label: 'Hello',
    // Display attributes:
    x: 0,
    y: 0,
    size: 1,
  }).addNode({
    // Main attributes:
    id: 'n1',
    label: 'World !',
    // Display attributes:
    x: 1,
    y: 1,
    size: 1,
    color: '#00f'
  }).addEdge({
    id: 'e0',
    // Reference extremities:
    source: 'n0',
    target: 'n1'
  });
s.settings({
    edgeColor: 'default',
    defaultNodeColor: '#999',
    defaultEdgeColor: '#999',
    defaultLabelSize: 14,
})
// s.graph.read(data)
s.refresh()

console.log("JS done")
