# Generate Markdown from Nodes

## Goal

The goal of this output format is to generate a markdown document from nodes all
defined with a layoutMode of bullets, but interpreted as headers, paragraphs or
as items of a list, depending on the context. This should allow the writer to
create a hierachical document in Workflowy withouth formatting and generate a
document with headers, paragraphs and lists in markdown format.

## Approach

When nodes have layoutMode other than "bullets", the markdown output will
respect that formatting.

When nodes have a layoutMode of "bullets", all nodes with children nodes will be treated
a headers, and all children nodes of those, will be treated as paragraphs.  However, if they have children of their own, then they will be treated as subheaders.

If a node has children nodes, but they are deteremined to be a list of items
(based on a heuristic), then the children nodes of that node will be treated as
a list of items.

Additionally, tags can be added at the end of nodes to specify the layout mode.
This allows the editor in Workflowy to truly work with non formatted text and
avoid the visual clutter it introduces.  Additional tags such as `#exclude` can
be used to exclude nodes from the output document all together.


## Examples

### Example 1

```
- Item A
  - Item A1
  - Item A2
- Item B
  - Item B1
  -
  - Item B2
- Item C
  - Item C1
    - Item C11
    - Item C12
  - Item C2
    - Item C21
    - Item C22
```
will produce:

```
# Item A
Item A1. Item A2.

# Item B
Item B1.

Item B2.

# Item C

## Item C1
Item C11. Item C12.

## Item C2
Item C21. Item C22.
```


### Example 2

```
- Item C
  - Item C1
  - Item C2
    - Item C21, a paragraph with some text
    - Item C22, another paragraph with more text
  - Item C2
    - Item C21, a third paragraph with so much text
    - Item C22, all these paragraphs
```
will produce:
```
# Item C

Item C1.

## Item C2

Item C21, a paragraph with some text. Item C22, another paragraph with more text.

## Item C2

Item C21, a third paragraph with so much text. Item C22, all these paragraphs.



### Example 3 - list detected over sub

```
- Item A
  - This is a rather lengthy paragraph
  - This looks like the beginning of a list:
    - item 1
    - item 2
    - item 3
  - This looks like another lengthy paragraph.
```
will produce:

```
# Item A

This is a rather lengthy paragraph. This looks like the beginning of a list:

- item 1
- item 2
- item 3

This looks like another lengthy paragraph.

```



## Testing

All examples above should be tested through unit tests.