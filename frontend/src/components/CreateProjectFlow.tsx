export default function CreateProjectFlow(props: {
  open: boolean
  onClose: () => void
  onCreated: () => void
}) {
  if (!props.open) return null
  return (
    <div className="modal-backdrop" onClick={props.onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Create Project</h3>
        </div>
        <p>Create project form will appear here</p>
      </div>
    </div>
  )
}
