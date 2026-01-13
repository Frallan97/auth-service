import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { organizationsAPI, Organization, OrganizationMember, AddMemberRequest } from '../../services/api'

const ROLES = [
  { value: 'owner', label: 'Owner', color: 'purple' },
  { value: 'admin', label: 'Admin', color: 'blue' },
  { value: 'member', label: 'Member', color: 'green' },
  { value: 'viewer', label: 'Viewer', color: 'gray' },
]

export default function OrganizationMembers() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [organization, setOrganization] = useState<Organization | null>(null)
  const [members, setMembers] = useState<OrganizationMember[]>([])
  const [loading, setLoading] = useState(true)
  const [showAddModal, setShowAddModal] = useState(false)
  const [editingMember, setEditingMember] = useState<OrganizationMember | null>(null)

  // Form state
  const [addFormData, setAddFormData] = useState<AddMemberRequest>({
    email: '',
    role: 'member',
  })
  const [editRole, setEditRole] = useState<string>('member')

  useEffect(() => {
    if (id) {
      loadOrganization()
      loadMembers()
    }
  }, [id])

  const loadOrganization = async () => {
    if (!id) return
    try {
      const data = await organizationsAPI.get(id)
      setOrganization(data)
    } catch (error) {
      console.error('Failed to load organization:', error)
      alert('Failed to load organization')
    }
  }

  const loadMembers = async () => {
    if (!id) return
    try {
      setLoading(true)
      const data = await organizationsAPI.listMembers(id)
      setMembers(data)
    } catch (error) {
      console.error('Failed to load members:', error)
      alert('Failed to load members')
    } finally {
      setLoading(false)
    }
  }

  const handleAddMember = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!id) return

    if (!addFormData.email) {
      alert('Email is required')
      return
    }

    try {
      await organizationsAPI.addMember(id, addFormData)
      setShowAddModal(false)
      resetAddForm()
      loadMembers()
    } catch (error: any) {
      console.error('Failed to add member:', error)
      if (error.response?.status === 404) {
        alert('User not found with that email')
      } else if (error.response?.status === 409) {
        alert('User is already a member of this organization')
      } else {
        alert('Failed to add member')
      }
    }
  }

  const handleUpdateRole = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!id || !editingMember) return

    try {
      await organizationsAPI.updateMember(id, editingMember.user_id, { role: editRole })
      setEditingMember(null)
      loadMembers()
    } catch (error: any) {
      console.error('Failed to update member:', error)
      if (error.response?.status === 400) {
        alert(error.response?.data?.error || 'Cannot update member role')
      } else {
        alert('Failed to update member role')
      }
    }
  }

  const handleRemoveMember = async (member: OrganizationMember) => {
    if (!id) return

    if (!confirm(`Remove ${member.name} (${member.email}) from this organization?`)) {
      return
    }

    try {
      await organizationsAPI.removeMember(id, member.user_id)
      loadMembers()
    } catch (error: any) {
      console.error('Failed to remove member:', error)
      if (error.response?.status === 400) {
        alert(error.response?.data?.error || 'Cannot remove member')
      } else {
        alert('Failed to remove member')
      }
    }
  }

  const startEditRole = (member: OrganizationMember) => {
    setEditingMember(member)
    setEditRole(member.role)
  }

  const resetAddForm = () => {
    setAddFormData({
      email: '',
      role: 'member',
    })
  }

  const getRoleColor = (role: string) => {
    const roleConfig = ROLES.find(r => r.value === role)
    return roleConfig?.color || 'gray'
  }

  const getRoleLabel = (role: string) => {
    const roleConfig = ROLES.find(r => r.value === role)
    return roleConfig?.label || role
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-lg text-gray-900 dark:text-white">Loading...</div>
      </div>
    )
  }

  return (
    <div className="max-w-7xl mx-auto">
      <div className="mb-8">
        <button
          onClick={() => navigate('/admin/organizations')}
          className="text-sm text-blue-600 dark:text-blue-400 hover:underline mb-4"
        >
          ← Back to Organizations
        </button>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2">
              {organization?.name} - Members
            </h1>
            <p className="text-sm text-gray-700 dark:text-gray-300">
              Total members: {members.length}
            </p>
          </div>
          <button
            onClick={() => setShowAddModal(true)}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            Add Member
          </button>
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 shadow overflow-hidden sm:rounded-lg">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
          <thead className="bg-gray-50 dark:bg-gray-900">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Member
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Email
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Role
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Joined
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
            {members.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-6 py-8 text-center text-gray-500 dark:text-gray-400">
                  No members yet. Add members to get started.
                </td>
              </tr>
            ) : (
              members.map((member) => (
                <tr key={member.id} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                  <td className="px-6 py-4">
                    <div className="flex items-center">
                      {member.avatar_url ? (
                        <img
                          src={member.avatar_url}
                          alt={member.name}
                          className="h-10 w-10 rounded-full mr-3"
                        />
                      ) : (
                        <div className="h-10 w-10 rounded-full bg-gray-300 dark:bg-gray-600 flex items-center justify-center text-gray-700 dark:text-gray-300 font-medium mr-3">
                          {member.name.charAt(0).toUpperCase()}
                        </div>
                      )}
                      <div className="text-sm font-medium text-gray-900 dark:text-white">
                        {member.name}
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    {member.email}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
                        ${getRoleColor(member.role) === 'purple' ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200' : ''}
                        ${getRoleColor(member.role) === 'blue' ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' : ''}
                        ${getRoleColor(member.role) === 'green' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : ''}
                        ${getRoleColor(member.role) === 'gray' ? 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200' : ''}
                      `}
                    >
                      {getRoleLabel(member.role)}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    {new Date(member.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button
                      onClick={() => startEditRole(member)}
                      className="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300 mr-4"
                    >
                      Change Role
                    </button>
                    <button
                      onClick={() => handleRemoveMember(member)}
                      className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Add Member Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-md">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">
              Add Member
            </h2>
            <form onSubmit={handleAddMember}>
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  User Email *
                </label>
                <input
                  type="email"
                  value={addFormData.email}
                  onChange={(e) => setAddFormData({ ...addFormData, email: e.target.value })}
                  placeholder="user@example.com"
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                  required
                />
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                  User must already be registered in the system
                </p>
              </div>

              <div className="mb-6">
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  Role *
                </label>
                <select
                  value={addFormData.role}
                  onChange={(e) => setAddFormData({ ...addFormData, role: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  {ROLES.map((role) => (
                    <option key={role.value} value={role.value}>
                      {role.label}
                    </option>
                  ))}
                </select>
                <div className="text-xs text-gray-500 dark:text-gray-400 mt-2 space-y-1">
                  <div><strong>Owner:</strong> Full control, can manage all members</div>
                  <div><strong>Admin:</strong> Can manage members except owners</div>
                  <div><strong>Member:</strong> Standard access</div>
                  <div><strong>Viewer:</strong> Read-only access</div>
                </div>
              </div>

              <div className="flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => {
                    setShowAddModal(false)
                    resetAddForm()
                  }}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  Add Member
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Edit Role Modal */}
      {editingMember && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-md">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">
              Change Role for {editingMember.name}
            </h2>
            <form onSubmit={handleUpdateRole}>
              <div className="mb-6">
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                  New Role *
                </label>
                <select
                  value={editRole}
                  onChange={(e) => setEditRole(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  {ROLES.map((role) => (
                    <option key={role.value} value={role.value}>
                      {role.label}
                    </option>
                  ))}
                </select>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-2">
                  Current role: <strong>{getRoleLabel(editingMember.role)}</strong>
                </p>
              </div>

              <div className="flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setEditingMember(null)}
                  className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  Update Role
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
