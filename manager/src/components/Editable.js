// Editable.js
import React, { useState, useEffect } from "react";

// Component accept text, placeholder values and also pass what type of Input - input, textarea so that we can use it for styling accordingly
const Editable = ({
  title,
  text,
  type,
  placeholder,
  onChange,
  onCancel,
  childRef,
  children,
  ...props
}) => {
  // Manage the state whether to show the label or the input box. By default, label will be shown.
// Exercise: It can be made dynamic by accepting initial state as props outside the component 
  const [isEditing, setEditing] = useState(false)

// Event handler while pressing any key while editing
  const handleKeyDown = (e) => {
    if (['Enter', 'Tab'].indexOf(e.code) >= 0 && onChange) onChange(e)
    if (['Escape'].indexOf(e.code) >= 0 && onCancel) onCancel(e)
    if (['Enter', 'Escape'].indexOf(e.code) >= 0) setEditing(false)
  }

  useEffect(() => {
    if (childRef && childRef.current && isEditing === true) {
      childRef.current.focus()
    }
  }, [isEditing, childRef])

/*
- It will display a label is `isEditing` is false
- It will display the children (input or textarea) if `isEditing` is true
- when input `onBlur`, we will set the default non edit mode
Note: For simplicity purpose, I removed all the classnames, you can check the repo for CSS styles
*/

  if (isEditing) return (
    <section {...props}>
        <div
          onBlur={() => setEditing(false)}
          onKeyDown={e => handleKeyDown(e, type)}
        >
          {title}: {children}
        </div>
    </section>
  )
  return (
    <section {...props}>
        <div onClick={() => setEditing(true)} >
          <span>
            {title}: <strong>{String(text)}</strong>
          </span>
        </div>
    </section>
  );
};

export default Editable;
