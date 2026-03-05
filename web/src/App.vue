<template>
  <router-view />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { watch } from 'vue'
import { useAppStore } from '@/stores'

const appStore = useAppStore()
const { locale } = useI18n()

watch(() => appStore.locale, val => { locale.value = val }, { immediate: true })
</script>
