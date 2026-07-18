import { describe, expect, it } from 'vitest'

import { createDefaultUploadForm } from '@/components/dashboard/wardrobe/uploadForm'

describe('createDefaultUploadForm', () => {
  it('returns exact default texture upload fields', () => {
    expect(createDefaultUploadForm()).toEqual({
      texture_type: 'skin',
      model: 'default',
      note: '',
      is_public: false,
      file: null,
    })
  })

  it('returns independent objects for each form reset', () => {
    const first = createDefaultUploadForm()
    const second = createDefaultUploadForm()

    first.note = 'changed'
    first.is_public = true

    expect(second).toEqual({
      texture_type: 'skin',
      model: 'default',
      note: '',
      is_public: false,
      file: null,
    })
  })
})
