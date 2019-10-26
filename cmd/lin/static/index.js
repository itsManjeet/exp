const g = new dagreD3.graphlib.Graph().setGraph({})

g.setNode('loading', { label: 'loading' })

const svg = d3.select('svg'), inner = svg.select('g')

// Set up zoom support
const zoom = d3.zoom().on("zoom", function() {
  inner.attr("transform", d3.event.transform)
})
svg.call(zoom)

// Create the renderer
const render = new dagreD3.render()

// Run the renderer. This is what draws the final graph.
render(inner, g)

// Center the graph
const initialScale = 0.5
svg.call(zoom.transform, d3.zoomIdentity.translate(20, 0).scale(initialScale))

svg.attr('height', g.graph().height * initialScale + 40)

const redraw = edges => {
  // Reset graph.
  g.nodes().forEach(n => g.removeNode(n))

  // Draw new graph.
  edges.forEach(e => {
    g.setNode(e.From, {label: e.From})
    g.setNode(e.To, {label: e.To})
    g.setEdge(e.From, e.To, {})
  })

  // Render.
  render(inner, g)

  // Add hovers.
  d3.select('svg')
    .selectAll('path')
    .on('mouseover', function(e) { // Must be a func to have correct 'this' scope.
      document.getElementById(`edgelist-${e.v}${e.w}`).style.backgroundColor = 'red'
      d3.select(this).style('stroke', 'red')
      d3.select(this).style('stroke-width', '5px')
    })
    .on('mouseout', function(e) { // Must be a func to have correct 'this' scope.
    document.getElementById(`edgelist-${e.v}${e.w}`).style.backgroundColor = 'transparent'
      d3.select(this).style('stroke', 'black')
      d3.select(this).style('stroke-width', '1.5px')
    })

  // List in text box.
  const edgeList = document.getElementsByClassName('edgeList')[0]
  edgeList.innerHTML = ''
  edges.forEach(e => {
    const newEdgeChild = document.createElement('div')
    newEdgeChild.id = `edgelist-${e.From}${e.To}`
    newEdgeChild.innerHTML = `${e.From} -> ${e.To}`
    newEdgeChild.onmouseover = _ => {
      newEdgeChild.style.backgroundColor = 'red'
      d3.select('svg')
        .selectAll('path')
        .filter(svgE => svgE.v == e.From && svgE.w == e.To )
        .each(function() {
          d3.select(this).style('stroke', 'red')
          d3.select(this).style('stroke-width', '5px')
        })
    }
    newEdgeChild.onmouseout = _ => {
      newEdgeChild.style.backgroundColor = 'transparent'
      d3.select('svg')
        .selectAll('path')
        .filter(svgE => svgE.v == e.From && svgE.w == e.To )
        .each(function() {
          d3.select(this).style('stroke', 'black')
          d3.select(this).style('stroke-width', '1.5px')
        })
    }
    edgeList.appendChild(newEdgeChild)
  })
}

fetch('/graph').then(function(resp) {
  resp.json().then(edges => {
    redraw(edges)
  })
}).catch(function(err) {
    console.error(err)
})
