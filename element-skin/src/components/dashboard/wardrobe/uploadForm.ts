export interface TextureUploadForm {
  texture_type: string
  model: string
  note: string
  is_public: boolean
  file: File | null
}

export function createDefaultUploadForm(): TextureUploadForm {
  return {
    texture_type: 'skin',
    model: 'default',
    note: '',
    is_public: false,
    file: null,
  }
}
