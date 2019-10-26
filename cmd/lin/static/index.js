const g = new dagreD3.graphlib.Graph().setGraph({})

g.setNode('loading', { label: 'loading' })

const svg = d3.select('svg'), inner = svg.select('g')

// Set up zoom support
const zoom = d3.zoom().on('zoom', function() {
  inner.attr('transform', d3.event.transform)
})
svg.call(zoom)

// Create the renderer
const render = new dagreD3.render()

// Run the renderer. This is what draws the final graph.
render(inner, g)

// Center the graph
const initialScale = 0.4
svg.call(zoom.transform, d3.zoomIdentity.translate(20, 0).scale(initialScale))

svg.attr('height', g.graph().height * initialScale + 40)

const redrawGraph = graph => {
  // Remove initial node.
  g.removeNode('loading')

  // Remove all edges not in graph.
  g.edges().forEach(e => {
    if (graph[e.v] == undefined) {
      g.removeEdge(e.v, e.w)
      // TODO: remove nodes too.
      return
    }
    if (graph[e.v][e.w] == undefined) {
      g.removeEdge(e.v, e.w)
      // TODO: remove nodes too.
      return
    }
  })

  // Draw new graph.
  Object.entries(graph).forEach(entry => {
    const from = entry[0]
    const tos = entry[1]
    for (const to in tos) {
      if (!g.hasNode(from) && !g.hasNode(to)) {
        g.setNode(from, {label: from})
        g.setNode(to, {label: to})
        g.setEdge(from, to, {})
      } else if (!g.hasNode(from)) {
        g.setNode(from, {label: from})
        g.setEdge(from, to, {})
      } else if (!g.hasNode(to)) {
        g.setNode(to, {label: to})
        g.setEdge(from, to, {})
      } else if (!g.hasEdge(from, to)) {
        g.setEdge(from, to, {})
      }
    }
  })

  // Render.
  render(inner, g)

  // Add hovers.
  d3.select('svg')
    .selectAll('path')
    .on('mouseover', function(e) { // Must be a func to have correct 'this' scope.
      document.getElementById(`edgeList-${e.v}${e.w}`).style.backgroundColor = 'red'
      d3.select(this).style('stroke', 'red')
      d3.select(this).style('stroke-width', '5px')
    })
    .on('mouseout', function(e) { // Must be a func to have correct 'this' scope.
      document.getElementById(`edgeList-${e.v}${e.w}`).style.backgroundColor = 'transparent'
      d3.select(this).style('stroke', 'black')
      d3.select(this).style('stroke-width', '1.5px')
    })
}

const drawList = (id, entries, clickMethod) => {
  const el = document.getElementById(id)
  el.innerHTML = ''

  Object.entries(entries).forEach(entry => {
    const from = entry[0]
    const tos = entry[1]
    for (const to in tos) {
      const newEdgeRow = document.createElement('div')

      const rowText = document.createElement('div')
      rowText.innerHTML = `${from} -> ${to}`
      rowText.className = 'left'
      newEdgeRow.appendChild(rowText)
  
      const rowButton = document.createElement('button')
      rowButton.type = 'button'
      if (clickMethod == 'POST') {
        rowButton.innerHTML = 'Return'
      } else {
        rowButton.innerHTML = 'Remove'
      }
      rowButton.className = 'right'
      rowButton.onclick = _ => {
        fetch('/edge', {method: clickMethod, body: JSON.stringify({'from': from, 'to': to})}).then(function(resp) {
          resp.json().then(both => {
            redrawGraph(both['graph'])
            redrawEdgelist(both['graph'])
            redrawShoppingCart(both['shoppingCart'])
          })
        }).catch(function(err) {
          console.error(err)
        })
      }
      newEdgeRow.appendChild(rowButton)
  
      newEdgeRow.id = `${id}-${from}${to}`
      newEdgeRow.className = 'edgeRow'
      newEdgeRow.dataset.from = from
      newEdgeRow.dataset.to = to
      newEdgeRow.onmouseover = _ => {
        newEdgeRow.classList.add('active')
        d3.select('svg')
          .selectAll('path')
          .filter(svgE => svgE.v == from && svgE.w == to )
          .each(function() {
            d3.select(this).style('stroke', 'red')
            d3.select(this).style('stroke-width', '5px')
          })
      }
      newEdgeRow.onmouseout = _ => {
        newEdgeRow.classList.remove('active')
        d3.select('svg')
          .selectAll('path')
          .filter(svgE => svgE.v == from && svgE.w == to )
          .each(function() {
            d3.select(this).style('stroke', 'black')
            d3.select(this).style('stroke-width', '1.5px')
          })
      }
      el.appendChild(newEdgeRow)
    }
  })
}

const redrawEdgelist = graph => {
  drawList('edgeList', graph, 'DELETE')
}

const redrawShoppingCart = shoppingCart => {
  drawList('shoppingCart', shoppingCart, 'POST')
}

document.getElementById('reset').onclick = _ => {
  fetch('/reset').then(function(resp) {
    resp.json().then(both => {
      redrawGraph(both['graph'])
      redrawEdgelist(both['graph'])
      redrawShoppingCart(both['shoppingCart'])
    })
  }).catch(function(err) {
      console.error(err)
  })
}

fetch('/graph').then(function(resp) {
  resp.json().then(graph => {
    redrawGraph(graph)
    redrawEdgelist(graph)
  })
}).catch(function(err) {
    console.error(err)
})

fetch('/shoppingCart').then(function(resp) {
  resp.json().then(shoppingCart => {
    redrawShoppingCart(shoppingCart)
  })
}).catch(function(err) {
    console.error(err)
})
