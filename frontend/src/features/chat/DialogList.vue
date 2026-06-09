<script setup lang="ts">
import { computed, ref } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import type { Dialog } from '@/types/chat'
import DialogItem from './DialogItem.vue'

const props = defineProps<{
  dialogs: Dialog[]
  selected: string | null
}>()

const emit = defineEmits<{ select: [ref: string] }>()

const searchQuery = ref('')
const filtered = computed(() => {
  if (!searchQuery.value) return props.dialogs
  const q = searchQuery.value.toLowerCase()
  return props.dialogs.filter(d =>
    d.title.toLowerCase().includes(q) ||
    (d.username || '').toLowerCase().includes(q)
  )
})

const scrollParent = ref<HTMLElement | null>(null)

const virtualizer = useVirtualizer({
  count: filtered.value.length,
  getScrollElement: () => scrollParent.value,
  estimateSize: () => 72,
  overscan: 5,
})
</script>

<template>
  <div class="dialog-list-wrapper">
    <div class="dialog-search">
      <input
        v-model="searchQuery"
        type="text"
        class="dialog-search-input"
        placeholder="搜索会话..."
      />
    </div>
    <div ref="scrollParent" class="dialog-scroll-container">
      <div :style="{ height: virtualizer.getTotalSize() + 'px', position: 'relative' }">
        <div
          v-for="row in virtualizer.getVirtualItems()"
          :key="String(row.key)"
          :style="{
            position: 'absolute',
            top: row.start + 'px',
            width: '100%',
          }"
        >
          <DialogItem
            :dialog="filtered[row.index]"
            :selected="filtered[row.index]?.peer_ref === selected"
            @click="emit('select', filtered[row.index].peer_ref)"
          />
        </div>
      </div>
    </div>
  </div>
</template>
