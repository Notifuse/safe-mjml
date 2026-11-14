# Container Width Flow in MJML Components

## Overview

MJML components use a cascading width calculation system where each parent component calculates the effective width available to its children after accounting for its own padding, borders, and margins.

## The Width Flow Chain

### 1. Section → Column
- **Section** provides its effective width (usually 600px) to columns
- **Column** receives parent width and calculates its content width by subtracting its own padding/borders
- **Column** sets this calculated width as `containerWidth` for all its children

### 2. Column → Child Components (Divider, Text, etc.)
- **Child components** receive `containerWidth` from column context
- **Child components** subtract their own padding to calculate final rendered width

## Example Calculation

Given this MJML structure:
```xml
<mj-section>  <!-- 600px width -->
  <mj-column mj-class="padded">  <!-- padding="20px" = 20px left + 20px right -->
    <mj-divider />  <!-- default padding="10px 25px" = 25px left + 25px right -->
  </mj-column>
</mj-section>
```

**Expected width flow:**
1. Section: 600px
2. Column effective width: 600px - 40px (column padding) = 560px
3. Column sets `containerWidth = 560px` for children
4. Divider width: 560px - 50px (divider padding) = **510px**

## MJML.io Reference Implementation

From the official MJML codebase (`packages/mjml-column/src/index.js` and `packages/mjml-divider/src/index.js`):

### Column Context (mjml-column/src/index.js)
```javascript
getChildContext() {
  // ... calculate allPaddings (left + right padding/borders) ...
  if (unit === '%') {
    containerWidth = `${
      (parseFloat(parentWidth) * parsedWidth) / 100 - allPaddings
    }px`
  } else {
    containerWidth = `${parsedWidth - allPaddings}px`
  }
  return {
    ...this.context,
    containerWidth,
  }
}
```

### Divider Width Calculation (mjml-divider/src/index.js)
```javascript
getOutlookWidth() {
  const { containerWidth } = this.context
  const paddingSize =
    this.getShorthandAttrValue('padding', 'left') +
    this.getShorthandAttrValue('padding', 'right')
  // ...
  return `${parseInt(containerWidth, 10) - paddingSize}px`
}
```

## How safe-mjml implements it

The widths flow down once, when the tree is built, as in MJML:

- `components.ChildContext` is what a container gives its children: the
  container width, the sibling counts, the flags `InsideGroup` and
  `InSection`, and, unexported, the child's index among its siblings and the
  gap of a parent mj-wrapper, as MJML's props.index and context.gap.
- `buildChildren` (`mjml/component.go`) computes it with
  `components.ChildContextOf` and sets it on each child with `SetContext`,
  before building the child's own children.
- By default a child gets its parent's own width. A container with a
  `childContext` method (`internal/components/context.go`) gives its own, like
  MJML's `getChildContext`: mj-section and mj-wrapper their box width
  (`boxWidth`, MJML's `getBoxWidths`), mj-column its content width, mj-group
  its width minus its left and right padding, mj-hero its width minus its
  padding.
- The same context carries the attributes a parent passes down
  (`PassedAttributes`), as MJML's `renderChildren` does; they reach only the
  direct children.
- Components read their width with `layoutWidth` (the width as passed,
  fractions included, as MJML's `parseFloat`) or `GetEffectiveWidth` (whole
  pixels, as MJML's `parseInt`), and the rest with `Context()`; nothing sets
  a width during `Render`.

## Debug Tips

1. **Add debug logging** to track width flow: Section → Column → Child
2. **Check MSO table widths** in generated HTML - they should match calculated widths
3. **Test with different padding combinations** to verify calculations
4. **Compare against MJML.io output** for the same template

## Detailed MJML.io Implementation Notes

### Column Width Calculation Details
- **Location**: `packages/mjml-column/src/index.js` in `getChildContext()` method
- **Box Model**: Uses `getBoxWidths()` helper to calculate total padding (left + right) and borders
- **Calculation**: 
  - Percentage width: `(parentWidth * percent) / 100 - allPaddings`
  - Pixel width: `parsedWidth - allPaddings`
- **Context**: Passes computed `containerWidth` as pixel string to children via React context

### Divider Width Usage Details  
- **Location**: `packages/mjml-divider/src/index.js` in `getOutlookWidth()` method
- **Process**:
  1. Gets `containerWidth` from parent context
  2. Calculates own left + right paddings
  3. Subtracts paddings from container width for effective width
  4. Applies percentage if width attribute specified
- **MSO Tables**: Uses `getOutlookWidth()` result for Outlook-specific table rendering

### Key Insight
The column **must** subtract paddings/borders before setting container width for children. The divider then subtracts its own paddings from the received container width.

## Related Files

- `internal/components/context.go` - `ChildContext`, `ChildContextOf` and the containers' `childContext`
- `mjml/component.go` - `buildChildren`, which sets the contexts
- `internal/components/shorthand.go` - `boxWidth` and the MJML readers of padding and borders
- `internal/components/column.go` - Column width and content width
- `internal/components/divider.go` - Divider MSO width calculation
