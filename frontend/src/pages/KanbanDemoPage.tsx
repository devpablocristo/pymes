/**
 * Kanban demo — inspirado en el template Wowdash, reconstruido con design tokens nativos.
 * Drag-and-drop: @hello-pangea/dnd (ya disponible en el repo).
 */
import { useState, useEffect, useCallback, type ReactNode } from 'react';
import { DragDropContext, Droppable, Draggable, type DropResult } from '@hello-pangea/dnd';
import './KanbanDemoPage.css';

// ─── Tipos ───

type Task = {
  id: string;
  title: string;
  description: string;
  tag: string;
  date: string;
  image: string | null;
};

type Column = {
  id: string;
  title: string;
  taskIds: string[];
};

type BoardData = {
  columns: Record<string, Column>;
  tasks: Record<string, Task>;
  columnOrder: string[];
};

// ─── Datos demo ───

const DEMO_DATA: BoardData = {
  columns: {
    'col-1': { id: 'col-1', title: 'En progreso', taskIds: ['t-1', 't-2'] },
    'col-2': { id: 'col-2', title: 'Pendiente', taskIds: ['t-3', 't-4'] },
    'col-3': { id: 'col-3', title: 'Completado', taskIds: ['t-5', 't-6', 't-7'] },
  },
  tasks: {
    't-1': {
      id: 't-1',
      title: 'Diseñar landing page',
      description: 'Crear wireframes y mockups para la nueva landing de producto.',
      tag: 'Diseño',
      date: '2026-03-28',
      image: null,
    },
    't-2': {
      id: 't-2',
      title: 'Integrar pasarela de pago',
      description: 'Conectar Stripe para pagos con tarjeta y transferencia.',
      tag: 'Backend',
      date: '2026-03-30',
      image: null,
    },
    't-3': {
      id: 't-3',
      title: 'Configurar CI/CD',
      description: 'Pipeline de GitHub Actions para build, test y deploy automático.',
      tag: 'DevOps',
      date: '2026-04-01',
      image: null,
    },
    't-4': {
      id: 't-4',
      title: 'Onboarding de usuarios',
      description: 'Flujo paso a paso para que nuevos usuarios configuren su cuenta.',
      tag: 'UX',
      date: '2026-04-02',
      image: null,
    },
    't-5': {
      id: 't-5',
      title: 'Auth con Clerk',
      description: 'Login, registro y gestión de sesiones con Clerk.',
      tag: 'Auth',
      date: '2026-03-20',
      image: null,
    },
    't-6': {
      id: 't-6',
      title: 'Dashboard principal',
      description: 'Vista resumen con métricas clave del negocio.',
      tag: 'Frontend',
      date: '2026-03-22',
      image: null,
    },
    't-7': {
      id: 't-7',
      title: 'API de clientes',
      description: 'CRUD completo de clientes con búsqueda y paginación.',
      tag: 'Backend',
      date: '2026-03-18',
      image: null,
    },
  },
  columnOrder: ['col-1', 'col-2', 'col-3'],
};

let nextId = 100;
function uid() {
  nextId += 1;
  return `t-${nextId}`;
}

// ─── Componentes ───

function TaskCard({
  task,
  onEdit,
  onDelete,
}: {
  task: Task;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="kd__card">
      {task.image && <img src={task.image} alt="" className="kd__card-img" />}
      <h4 className="kd__card-title">{task.title}</h4>
      <p className="kd__card-desc">{task.description}</p>
      <span className="kd__card-tag">{task.tag}</span>
      <div className="kd__card-footer">
        <span className="kd__card-date">
          📅{' '}
          {new Date(task.date).toLocaleDateString('es-AR', {
            day: '2-digit',
            month: 'short',
            year: 'numeric',
          })}
        </span>
        <div className="kd__card-actions">
          <button type="button" className="kd__card-action kd__card-action--edit" onClick={onEdit} title="Editar">
            ✏️
          </button>
          <button type="button" className="kd__card-action kd__card-action--delete" onClick={onDelete} title="Eliminar">
            🗑️
          </button>
        </div>
      </div>
    </div>
  );
}

function TaskModal({
  task,
  onSave,
  onClose,
}: {
  task: Task | null;
  onSave: (t: Omit<Task, 'id'>, isEdit: boolean) => void;
  onClose: () => void;
}) {
  const isEdit = task !== null;
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [tag, setTag] = useState('');
  const [date, setDate] = useState('');
  const [imagePreview, setImagePreview] = useState('');

  useEffect(() => {
    if (task) {
      setTitle(task.title);
      setDescription(task.description);
      setTag(task.tag);
      setDate(task.date);
      setImagePreview(task.image ?? '');
    } else {
      setTitle('');
      setDescription('');
      setTag('');
      setDate(new Date().toISOString().slice(0, 10));
      setImagePreview('');
    }
  }, [task]);

  const handleImage = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onloadend = () => setImagePreview(reader.result as string);
    reader.readAsDataURL(file);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() || !description.trim()) return;
    onSave(
      { title: title.trim(), description: description.trim(), tag: tag.trim() || 'General', date, image: imagePreview || null },
      isEdit,
    );
  };

  return (
    <div className="kd__backdrop" onClick={onClose}>
      <form className="kd__modal" onClick={(e) => e.stopPropagation()} onSubmit={handleSubmit}>
        <div className="kd__modal-header">
          <h3 className="kd__modal-title">{isEdit ? 'Editar tarea' : 'Nueva tarea'}</h3>
          <button type="button" className="kd__modal-close" onClick={onClose}>
            ✕
          </button>
        </div>
        <div className="kd__modal-body">
          <div className="form-group">
            <label htmlFor="kd-title">Título</label>
            <input id="kd-title" type="text" value={title} onChange={(e) => setTitle(e.target.value)} required />
          </div>
          <div className="form-group">
            <label htmlFor="kd-tag">Etiqueta</label>
            <input id="kd-tag" type="text" value={tag} onChange={(e) => setTag(e.target.value)} placeholder="Ej: Diseño, Backend…" />
          </div>
          <div className="form-group">
            <label htmlFor="kd-date">Fecha</label>
            <input id="kd-date" type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
          </div>
          <div className="form-group">
            <label htmlFor="kd-desc">Descripción</label>
            <textarea id="kd-desc" rows={3} value={description} onChange={(e) => setDescription(e.target.value)} required />
          </div>
          <div className="form-group">
            <label htmlFor="kd-img">Imagen (opcional)</label>
            <input id="kd-img" type="file" accept="image/*" onChange={handleImage} />
            {imagePreview && <img src={imagePreview} alt="" className="kd__modal-img-preview" />}
          </div>
        </div>
        <div className="kd__modal-footer">
          <button type="button" className="btn-secondary btn-sm" onClick={onClose}>
            Cancelar
          </button>
          <button type="submit" className="btn-primary btn-sm">
            {isEdit ? 'Guardar' : 'Crear'}
          </button>
        </div>
      </form>
    </div>
  );
}

function KanbanColumn({
  column,
  tasks,
  onAddTask,
  onEditTask,
  onDeleteTask,
  isDragging,
}: {
  column: Column;
  tasks: Task[];
  onAddTask: (colId: string) => void;
  onEditTask: (taskId: string, colId: string) => void;
  onDeleteTask: (taskId: string, colId: string) => void;
  isDragging: boolean;
}) {
  return (
    <div className="kd__col">
      <Droppable droppableId={column.id}>
        {(provided, snapshot) => (
          <div
            className={`kd__col-body ${snapshot.isDraggingOver ? 'kd__col-body--over' : ''}`}
            ref={provided.innerRef}
            {...provided.droppableProps}
          >
            <div className="kd__col-head">
              <h3 className="kd__col-title">{column.title}</h3>
              <span className="kd__col-count">{tasks.length}</span>
            </div>
            <div className="kd__col-scroll">
              {tasks.map((task, index) => (
                <Draggable key={task.id} draggableId={task.id} index={index}>
                  {(prov, snap) => (
                    <div
                      ref={prov.innerRef}
                      {...prov.draggableProps}
                      {...prov.dragHandleProps}
                      className={snap.isDragging ? 'kd__card--dragging' : ''}
                      style={prov.draggableProps.style}
                    >
                      <TaskCard
                        task={task}
                        onEdit={() => onEditTask(task.id, column.id)}
                        onDelete={() => onDeleteTask(task.id, column.id)}
                      />
                    </div>
                  )}
                </Draggable>
              ))}
              {provided.placeholder}
              {isDragging && tasks.length === 0 && <div className="kd__drop-hint">Soltar aquí</div>}
            </div>
            <button type="button" className="kd__add-btn" onClick={() => onAddTask(column.id)}>
              + Agregar tarea
            </button>
          </div>
        )}
      </Droppable>
    </div>
  );
}

// ─── Página principal ───

export function KanbanDemoPage() {
  const [data, setData] = useState<BoardData>(DEMO_DATA);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [targetColumn, setTargetColumn] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);

  const onDragStart = useCallback(() => setIsDragging(true), []);

  const onDragEnd = useCallback(
    (result: DropResult) => {
      setIsDragging(false);
      const { destination, source, draggableId } = result;
      if (!destination) return;
      if (destination.droppableId === source.droppableId && destination.index === source.index) return;

      const startCol = data.columns[source.droppableId];
      const endCol = data.columns[destination.droppableId];

      if (startCol === endCol) {
        const ids = Array.from(startCol.taskIds);
        ids.splice(source.index, 1);
        ids.splice(destination.index, 0, draggableId);
        setData((prev) => ({
          ...prev,
          columns: { ...prev.columns, [startCol.id]: { ...startCol, taskIds: ids } },
        }));
        return;
      }

      const startIds = Array.from(startCol.taskIds);
      startIds.splice(source.index, 1);
      const endIds = Array.from(endCol.taskIds);
      endIds.splice(destination.index, 0, draggableId);
      setData((prev) => ({
        ...prev,
        columns: {
          ...prev.columns,
          [startCol.id]: { ...startCol, taskIds: startIds },
          [endCol.id]: { ...endCol, taskIds: endIds },
        },
      }));
    },
    [data.columns],
  );

  const handleAddTask = (colId: string) => {
    setEditingTask(null);
    setTargetColumn(colId);
    setModalOpen(true);
  };

  const handleEditTask = (taskId: string, _colId: string) => {
    setEditingTask(data.tasks[taskId]);
    setTargetColumn(null);
    setModalOpen(true);
  };

  const handleDeleteTask = (taskId: string, colId: string) => {
    setData((prev) => {
      const col = prev.columns[colId];
      const newTasks = { ...prev.tasks };
      delete newTasks[taskId];
      return {
        ...prev,
        columns: { ...prev.columns, [colId]: { ...col, taskIds: col.taskIds.filter((id) => id !== taskId) } },
        tasks: newTasks,
      };
    });
  };

  const handleSaveTask = (taskData: Omit<Task, 'id'>, isEdit: boolean) => {
    if (isEdit && editingTask) {
      setData((prev) => ({
        ...prev,
        tasks: { ...prev.tasks, [editingTask.id]: { ...editingTask, ...taskData } },
      }));
    } else if (targetColumn) {
      const newId = uid();
      setData((prev) => {
        const col = prev.columns[targetColumn];
        return {
          ...prev,
          tasks: { ...prev.tasks, [newId]: { id: newId, ...taskData } },
          columns: { ...prev.columns, [targetColumn]: { ...col, taskIds: [...col.taskIds, newId] } },
        };
      });
    }
    setModalOpen(false);
    setEditingTask(null);
    setTargetColumn(null);
  };

  return (
    <div className="kd">
      <div className="page-header">
        <h1>Kanban</h1>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.88rem' }}>
          Tablero de tareas con drag &amp; drop — demo inspirado en Wowdash
        </p>
      </div>

      <DragDropContext onDragStart={onDragStart} onDragEnd={onDragEnd}>
        <div className="kd__board">
          {data.columnOrder.map((colId) => {
            const col = data.columns[colId];
            const tasks = col.taskIds.map((tid) => data.tasks[tid]).filter(Boolean);
            return (
              <KanbanColumn
                key={col.id}
                column={col}
                tasks={tasks}
                onAddTask={handleAddTask}
                onEditTask={handleEditTask}
                onDeleteTask={handleDeleteTask}
                isDragging={isDragging}
              />
            );
          })}
        </div>
      </DragDropContext>

      {modalOpen && <TaskModal task={editingTask} onSave={handleSaveTask} onClose={() => setModalOpen(false)} />}
    </div>
  );
}

export default KanbanDemoPage;
