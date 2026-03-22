<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Events } from '@wailsio/runtime'
import { toast } from '@/components/ui/toast'
import { ChannelService, UpdateChannelInput } from '@bindings/chatclaw/internal/services/channels'

const { t } = useI18n()

// WhatsApp QR auth state
const qrDialogOpen = ref(false)
const qrCodeImage = ref('')
const qrChannelId = ref<number | null>(null)
let qrIsConnected = false

let qrUnsubscribe: (() => void) | null = null
let connectedUnsubscribe: (() => void) | null = null

watch(qrDialogOpen, async (isOpen) => {
  if (!isOpen && !qrIsConnected && qrChannelId.value) {
    try {
      await ChannelService.UpdateChannel(
        qrChannelId.value,
        new UpdateChannelInput({ enabled: false })
      )
      await ChannelService.DisconnectChannel(qrChannelId.value)
      // Notify application that the connection was aborted
      Events.Emit('channel:whatsapp:aborted', { channel_id: qrChannelId.value })
    } catch (e) {
      console.error('Failed to disable WhatsApp channel on QR close', e)
    }
    qrChannelId.value = null
  }
})

onMounted(() => {
  // Listen for WhatsApp QR code event
  qrUnsubscribe = Events.On('channel:whatsapp:qr', (event) => {
    if (event && event.data && event.data.qr_code) {
      qrCodeImage.value = event.data.qr_code as string
      qrChannelId.value = event.data.channel_id as number
      qrIsConnected = false
      if (!qrDialogOpen.value) {
        qrDialogOpen.value = true
      }
    }
  })

  // Listen for WhatsApp connected event
  connectedUnsubscribe = Events.On('channel:whatsapp:connected', (event) => {
    qrIsConnected = true
    qrDialogOpen.value = false
    qrCodeImage.value = ''
    qrChannelId.value = null
    toast.success(t('channels.whatsapp.connected', 'WhatsApp connected successfully!'))
  })
})

onUnmounted(() => {
  if (qrUnsubscribe) qrUnsubscribe()
  if (connectedUnsubscribe) connectedUnsubscribe()
})
</script>

<template>
  <Dialog v-model:open="qrDialogOpen">
    <DialogContent class="sm:max-w-[400px]">
      <DialogHeader>
        <DialogTitle>{{ t('channels.whatsapp.scanQr', 'Scan WhatsApp QR Code') }}</DialogTitle>
      </DialogHeader>
      <div class="flex flex-col items-center justify-center p-6">
        <img
          v-if="qrCodeImage"
          :src="qrCodeImage"
          class="h-64 w-64 object-contain mb-4 border border-border rounded-lg"
        />
        <p class="text-sm text-center text-muted-foreground">
          {{
            t(
              'channels.whatsapp.scanQrDesc',
              'Open WhatsApp on your phone and scan this QR code to link your account.'
            )
          }}
        </p>
      </div>
    </DialogContent>
  </Dialog>
</template>
