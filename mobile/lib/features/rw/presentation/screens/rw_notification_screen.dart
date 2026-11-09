import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/utils/url_launcher_utils.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';

/// RW Notification screen (Android). Contracts:
/// - Routes behind backend RequireRWScope; service enforces rw.notification.read/write.
/// - Explicit RW membership: read for pengurus RW, write for ketua/bendahara.
/// - rt_inferred / no RW membership: must not access (tab hidden; backend denies).
/// - Super Admin bypass handled by backend.
/// State is keyed by rwId/page via autoDispose family providers; RW/account change disposes cache.
class RwNotificationScreen extends ConsumerStatefulWidget {
  final String rwId;
  const RwNotificationScreen({super.key, required this.rwId});

  @override
  ConsumerState<RwNotificationScreen> createState() => _RwNotificationScreenState();
}

class _RwNotificationScreenState extends ConsumerState<RwNotificationScreen> {
  int _page = 1;
  bool _sending = false;
  bool _templateBusy = false;

  Widget _buildBodyWithLinks(String body) {
    final segments = UrlDetector.parseSegments(body);
    if (segments.isEmpty || segments.every((s) => !s.isLink)) {
      return Text(
        body,
        style: const TextStyle(color: Color(0xFFCBD5E1), fontSize: 12, height: 1.4),
      );
    }
    return RichText(
      text: TextSpan(
        style: const TextStyle(color: Color(0xFFCBD5E1), fontSize: 12, height: 1.4),
        children: segments.map((seg) {
          if (seg.isLink) {
            return TextSpan(
              text: seg.text,
              style: const TextStyle(
                color: Color(0xFFA970FF),
                decoration: TextDecoration.underline,
              ),
              recognizer: TapGestureRecognizer()..onTap = () => UrlDetector.openUrl(seg.url!),
            );
          }
          return TextSpan(text: seg.text);
        }).toList(),
      ),
    );
  }

  Widget _buildBodyWithLinksDialog(String body) {
    final segments = UrlDetector.parseSegments(body);
    if (segments.isEmpty || segments.every((s) => !s.isLink)) {
      return Text(
        body,
        style: const TextStyle(fontSize: 12, color: Colors.black87),
      );
    }
    return RichText(
      text: TextSpan(
        style: const TextStyle(fontSize: 12, color: Colors.black87),
        children: segments.map((seg) {
          if (seg.isLink) {
            return TextSpan(
              text: seg.text,
              style: const TextStyle(
                color: Color(0xFFA970FF),
                decoration: TextDecoration.underline,
              ),
              recognizer: TapGestureRecognizer()..onTap = () => UrlDetector.openUrl(seg.url!),
            );
          }
          return TextSpan(text: seg.text);
        }).toList(),
      ),
    );
  }

  bool _canWrite(Map<String, dynamic>? user) {
    if (user == null) return false;
    if (user['role'] == 'super_admin' || user['system_role'] == 'super_admin') return true;
    final memberships = (user['rw_memberships'] as List?)?.whereType<Map<String, dynamic>>() ?? const [];
    final m = memberships.where((e) => e['rw_id'] == widget.rwId).firstOrNull;
    if (m == null) return false;
    if ((m['source'] as String?) != 'explicit') return false;
    final j = ((m['jabatan'] as String?) ?? '').toLowerCase();
    return j == 'ketua' || j == 'bendahara';
  }

  String _friendlyError(Object e) {
    final s = e.toString();
    if (s.contains('403') || s.toLowerCase().contains('forbidden') || s.toLowerCase().contains('access denied') || s.toLowerCase().contains('permission denied')) {
      return 'Anda tidak memiliki izin untuk aksi ini.';
    }
    if (s.contains('401') || s.toLowerCase().contains('unauthorized')) {
      return 'Sesi tidak valid. Silakan login ulang.';
    }
    return 'Gagal memuat data notifikasi.';
  }

  Future<void> _refresh() async {
    ref.invalidate(rwNotificationLogsProvider((rwId: widget.rwId, page: _page, eventType: null)));
    ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
  }

  Future<void> _sendManualPush() async {
    final titleC = TextEditingController();
    final bodyC = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Kirim Notifikasi Manual'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(controller: titleC, decoration: const InputDecoration(labelText: 'Judul')),
            const SizedBox(height: 8),
            TextField(controller: bodyC, decoration: const InputDecoration(labelText: 'Isi'), maxLines: 3),
            const SizedBox(height: 8),
            const Text('Notifikasi akan dikirim ke pengurus RW dan RT di bawah RW ini.',
                style: TextStyle(fontSize: 12, color: Colors.black54)),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Batal')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Kirim')),
        ],
      ),
    );
    if (ok != true) return;
    final title = titleC.text.trim();
    final body = bodyC.text.trim();
    if (title.isEmpty || body.isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Judul dan isi wajib diisi')));
      return;
    }
    setState(() => _sending = true);
    try {
      final count = await ref.read(rwApiClientProvider).sendManualPush(widget.rwId, title: title, body: body);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Notifikasi terkirim ($count penerima)')));
      await _refresh();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  Future<void> _createTemplate() async {
    final nameC = TextEditingController();
    final titleC = TextEditingController();
    final bodyC = TextEditingController();
    String eventType = 'manual_push';
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(builder: (ctx, setS) => AlertDialog(
        title: const Text('Buat Template'),
        content: SingleChildScrollView(child: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(controller: nameC, decoration: const InputDecoration(labelText: 'Nama template')),
          DropdownButtonFormField<String>(
            value: eventType,
            decoration: const InputDecoration(labelText: 'Jenis event'),
            items: const [
              DropdownMenuItem(value: 'manual_push', child: Text('Notifikasi Manual')),
              DropdownMenuItem(value: 'iuran_payment_reminder', child: Text('Pengingat Iuran')),
            ],
            onChanged: (v) => setS(() => eventType = v ?? 'manual_push'),
          ),
          TextField(controller: titleC, decoration: const InputDecoration(labelText: 'Judul')),
          TextField(controller: bodyC, decoration: const InputDecoration(labelText: 'Isi'), maxLines: 3),
        ])),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Batal')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Simpan')),
        ],
      )),
    );
    if (ok != true) return;
    if (nameC.text.trim().isEmpty || titleC.text.trim().isEmpty || bodyC.text.trim().isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Semua field wajib diisi')));
      return;
    }
    setState(() => _templateBusy = true);
    try {
      await ref.read(rwApiClientProvider).createNotificationTemplate(
            widget.rwId,
            name: nameC.text.trim(),
            eventType: eventType,
            title: titleC.text.trim(),
            body: bodyC.text.trim(),
          );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Template dibuat')));
      ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _templateBusy = false);
    }
  }

  Future<void> _toggleTemplate(RwNotificationTemplate t) async {
    setState(() => _templateBusy = true);
    try {
      await ref.read(rwApiClientProvider).toggleNotificationTemplate(widget.rwId, t.id);
      ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _templateBusy = false);
    }
  }

  Future<void> _deleteTemplate(RwNotificationTemplate t) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Hapus template?'),
        content: Text(t.name),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Batal')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Hapus')),
        ],
      ),
    );
    if (ok != true) return;
    setState(() => _templateBusy = true);
    try {
      await ref.read(rwApiClientProvider).deleteNotificationTemplate(widget.rwId, t.id);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Template dihapus')));
      ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _templateBusy = false);
    }
  }

  Future<void> _previewTemplate(RwNotificationTemplate t, bool canWrite) async {
    showDialog(
      context: context,
      builder: (ctx) => FutureBuilder<Map<String, dynamic>>(
        future: ref.read(rwApiClientProvider).previewNotificationTemplate(widget.rwId, t.id),
        builder: (ctx, snap) {
          if (snap.connectionState == ConnectionState.waiting) {
            return const AlertDialog(
              content: SizedBox(
                height: 100,
                child: Center(child: CircularProgressIndicator()),
              ),
            );
          }
          final data = snap.data;
          final renderedTitle = data?['rendered_title'] as String? ?? t.title;
          final renderedBody = data?['rendered_body'] as String? ?? t.body;
          final sampleData = (data?['sample_data'] as Map?)?.cast<String, dynamic>() ?? const {};
          final placeholders = (data?['placeholders'] as List?)?.map((e) => '$e').toList() ?? const [];

          return AlertDialog(
            title: Row(
              children: [
                const Icon(Icons.preview_rounded, size: 20),
                const SizedBox(width: 8),
                Expanded(child: Text('Preview: ${t.name}', style: const TextStyle(fontSize: 16))),
              ],
            ),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Chip(
                        label: Text(t.eventType, style: const TextStyle(fontSize: 11)),
                        padding: EdgeInsets.zero,
                        materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      ),
                      const SizedBox(width: 8),
                      Text(
                        placeholders.isNotEmpty
                            ? '${placeholders.length} variabel'
                            : 'Tanpa variabel dinamis',
                        style: const TextStyle(fontSize: 11, color: Colors.black54),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  // Push notification simulation banner
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: const Color(0xFF0F172A),
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: const Color(0xFF334155)),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Container(
                              width: 20,
                              height: 20,
                              decoration: BoxDecoration(
                                color: Colors.blue.shade600,
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: const Icon(Icons.notifications_active, size: 12, color: Colors.white),
                            ),
                            const SizedBox(width: 6),
                            const Expanded(
                              child: Text(
                                'SOCIAL FINANCE URENA • RW',
                                style: TextStyle(color: Color(0xFF94A3B8), fontSize: 10, fontWeight: FontWeight.bold),
                              ),
                            ),
                            const Text('Baru saja', style: TextStyle(color: Color(0xFF64748B), fontSize: 10)),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Text(
                          renderedTitle,
                          style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.bold),
                        ),
                        const SizedBox(height: 4),
                        _buildBodyWithLinks(renderedBody),
                      ],
                    ),
                  ),
                  if (sampleData.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    const Text('Data Contoh Render:', style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: Colors.black87)),
                    const SizedBox(height: 4),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      children: sampleData.entries.map((e) => Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(color: Colors.grey.shade100, borderRadius: BorderRadius.circular(4)),
                        child: Text('{{${e.key}}}: ${e.value}', style: const TextStyle(fontSize: 10, color: Colors.black54)),
                      )).toList(),
                    ),
                  ],
                ],
              ),
            ),
            actions: [
              TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Tutup')),
              if (canWrite && t.isActive)
                FilledButton.icon(
                  onPressed: () {
                    Navigator.pop(ctx);
                    _sendTemplate(t);
                  },
                  icon: const Icon(Icons.send_rounded, size: 16),
                  label: const Text('Lanjut Kirim'),
                ),
            ],
          );
        },
      ),
    );
  }

  Future<void> _sendTemplate(RwNotificationTemplate t) async {
    String target = 'all_pengurus';
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) => AlertDialog(
          title: Text('Kirim: ${t.name}'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('Target Penerima:', style: TextStyle(fontSize: 12, fontWeight: FontWeight.bold)),
              const SizedBox(height: 6),
              DropdownButtonFormField<String>(
                value: target,
                decoration: const InputDecoration(border: OutlineInputBorder(), isDense: true),
                items: const [
                  DropdownMenuItem(value: 'all_pengurus', child: Text('Semua Pengurus (RW & RT)')),
                  DropdownMenuItem(value: 'rw_only', child: Text('Pengurus RW Saja')),
                ],
                onChanged: (v) => setS(() => target = v ?? 'all_pengurus'),
              ),
              const SizedBox(height: 12),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(color: Colors.grey.shade50, borderRadius: BorderRadius.circular(8), border: Border.all(color: Colors.black12)),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(t.title, style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13)),
                    const SizedBox(height: 4),
                    _buildBodyWithLinksDialog(t.body),
                  ],
                ),
              ),
              const SizedBox(height: 8),
              const Text('Notifikasi akan dikirim ke target terpilih dalam cakupan RW ini.',
                  style: TextStyle(fontSize: 11, color: Colors.black54)),
            ],
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Batal')),
            FilledButton.icon(
              onPressed: () => Navigator.pop(ctx, true),
              icon: const Icon(Icons.send_rounded, size: 16),
              label: const Text('Kirim Sekarang'),
            ),
          ],
        ),
      ),
    );
    if (ok != true) return;
    setState(() => _sending = true);
    try {
      final count = await ref.read(rwApiClientProvider).sendTemplateNotification(widget.rwId, t.id, target: target);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Notifikasi terkirim ($count penerima)')));
      await _refresh();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final canWrite = _canWrite(user);
    final logsAsync = ref.watch(rwNotificationLogsProvider((rwId: widget.rwId, page: _page, eventType: null)));
    final templatesAsync = ref.watch(rwNotificationTemplatesProvider(widget.rwId));

    return RefreshIndicator(
      onRefresh: _refresh,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Row(children: [
            const Expanded(child: Text('Notifikasi RW', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700))),
            if (canWrite)
              FilledButton.icon(
                key: const Key('rwNotifManualPushBtn'),
                onPressed: _sending ? null : _sendManualPush,
                icon: const Icon(Icons.send),
                label: const Text('Kirim Manual'),
              ),
          ]),
          const SizedBox(height: 4),
          const Text('Riwayat notifikasi dan template untuk RW ini.', style: TextStyle(color: Colors.black54)),
          const SizedBox(height: 16),
          _sectionTitle('Riwayat Notifikasi'),
          logsAsync.when(
            loading: () => const Padding(padding: EdgeInsets.all(24), child: Center(child: CircularProgressIndicator())),
            error: (e, _) => _errorBox(e),
            data: (pageData) {
              if (pageData.items.isEmpty) {
                return const _EmptyBox(message: 'Belum ada notifikasi untuk RW ini.');
              }
              return Column(children: [
                ...pageData.items.map((n) => _logTile(n)),
                if (pageData.totalPages > 1)
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                    IconButton(onPressed: _page > 1 ? () => setState(() => _page--) : null, icon: const Icon(Icons.chevron_left)),
                    Text('$_page / ${pageData.totalPages}'),
                    IconButton(onPressed: _page < pageData.totalPages ? () => setState(() => _page++) : null, icon: const Icon(Icons.chevron_right)),
                  ]),
              ]);
            },
          ),
          const SizedBox(height: 20),
          Row(children: [
            Expanded(child: _sectionTitle('Template Notifikasi')),
            if (canWrite)
              TextButton.icon(
                key: const Key('rwNotifCreateTemplateBtn'),
                onPressed: _templateBusy ? null : _createTemplate,
                icon: const Icon(Icons.add),
                label: const Text('Template'),
              ),
          ]),
          templatesAsync.when(
            loading: () => const Padding(padding: EdgeInsets.all(16), child: Center(child: CircularProgressIndicator())),
            error: (e, _) => _errorBox(e),
            data: (items) {
              if (items.isEmpty) {
                return _EmptyBox(message: canWrite ? 'Belum ada template. Buat template baru.' : 'Belum ada template yang tersedia.');
              }
              return Column(children: items.map((t) => _templateTile(t, canWrite)).toList());
            },
          ),
        ],
      ),
    );
  }

  Widget _sectionTitle(String t) => Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Text(t, style: const TextStyle(fontWeight: FontWeight.w700)),
      );

  Widget _errorBox(Object e) => Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(color: Colors.red.shade50, borderRadius: BorderRadius.circular(8)),
        child: Row(children: [
          const Icon(Icons.error_outline, color: Colors.red),
          const SizedBox(width: 8),
          Expanded(child: Text(_friendlyError(e))),
          TextButton(onPressed: _refresh, child: const Text('Coba lagi')),
        ]),
      );

  Widget _logTile(RwNotificationLog n) {
    final when = n.createdAt != null ? '${n.createdAt}'.split('.').first : '-';
    return Card(
      key: Key('rwNotifLog_${n.id}'),
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        title: Text(n.title?.isNotEmpty == true ? n.title! : n.eventType),
        subtitle: Text('${n.body ?? ''}\n$when', maxLines: 3, overflow: TextOverflow.ellipsis),
        isThreeLine: true,
        trailing: n.status != null && n.status!.isNotEmpty
            ? Chip(label: Text(n.status!, style: const TextStyle(fontSize: 11)))
            : null,
      ),
    );
  }

  Widget _templateTile(RwNotificationTemplate t, bool canWrite) {
    return Card(
      key: Key('rwNotifTemplate_${t.id}'),
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        title: Text(t.name),
        subtitle: Text('${t.eventType}\n${t.title}', maxLines: 2, overflow: TextOverflow.ellipsis),
        isThreeLine: true,
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            IconButton(
              key: Key('rwNotifTplPreview_${t.id}'),
              icon: const Icon(Icons.visibility_outlined),
              tooltip: 'Preview',
              onPressed: () => _previewTemplate(t, canWrite),
            ),
            if (canWrite) ...[
              IconButton(
                key: Key('rwNotifTplSend_${t.id}'),
                icon: const Icon(Icons.send_rounded, color: Colors.blue),
                tooltip: 'Kirim',
                onPressed: (!t.isActive || _sending) ? null : () => _sendTemplate(t),
              ),
              Switch(
                key: Key('rwNotifTplToggle_${t.id}'),
                value: t.isActive,
                onChanged: _templateBusy ? null : (_) => _toggleTemplate(t),
              ),
              IconButton(
                key: Key('rwNotifTplDelete_${t.id}'),
                icon: const Icon(Icons.delete_outline),
                onPressed: _templateBusy ? null : () => _deleteTemplate(t),
              ),
            ] else ...[
              Chip(label: Text(t.isActive ? 'Aktif' : 'Nonaktif', style: const TextStyle(fontSize: 11))),
            ],
          ],
        ),
      ),
    );
  }
}

class _EmptyBox extends StatelessWidget {
  final String message;
  const _EmptyBox({required this.message});
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(24),
      alignment: Alignment.center,
      decoration: BoxDecoration(border: Border.all(color: Colors.black12), borderRadius: BorderRadius.circular(8)),
      child: Text(message, textAlign: TextAlign.center, style: const TextStyle(color: Colors.black54)),
    );
  }
}
