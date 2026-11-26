#include <libproc.h>
#include <sys/sysctl.h>
#include <sys/resource.h>
#include <mach/mach_time.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <time.h>
extern int responsibility_get_pid_responsible_for_pid(pid_t);
struct proc_pidcoalinfo { uint64_t coalition_id[2]; uint64_t reserved[6]; }; // guess; verify below
static double now(){struct timespec t;clock_gettime(CLOCK_MONOTONIC,&t);return t.tv_sec*1e3+t.tv_nsec/1e6;}
int main(int argc,char**argv){
 int verbose=argc>1;
 mach_timebase_info_data_t tb; mach_timebase_info(&tb); printf("timebase %u/%u\n",tb.numer,tb.denom);
 int n=proc_listpids(PROC_ALL_PIDS,0,NULL,0); pid_t*pids=malloc(n*2); n=proc_listpids(PROC_ALL_PIDS,0,pids,n*2)/sizeof(pid_t);
 printf("pids %d uid %d\n",n,getuid());
 int ok_ru=0,e_ru=0,ok_ti=0,e_ti=0,ok_path=0,e_path=0,ok_args=0,e_args=0,ok_resp=0,ok_bsd=0,e_bsd=0,ok_co=0,e_co=0, e_ru_mine=0;
 double t0=now();
 double tl=0,tr=0,tt=0,tp=0,ta=0,tb2=0,tre=0,tc=0;
 for(int i=0;i<n;i++){ pid_t p=pids[i]; if(!p)continue; double a,b;
  struct rusage_info_v6 ru; a=now(); int r=proc_pid_rusage(p,RUSAGE_INFO_V6,(rusage_info_t*)&ru); tr+=now()-a; if(r==0)ok_ru++; else {e_ru++; if(verbose&&e_ru<4)printf("rusage pid %d: %s\n",p,strerror(errno));}
  struct proc_taskinfo ti; a=now(); r=proc_pidinfo(p,PROC_PIDTASKINFO,0,&ti,sizeof ti); tt+=now()-a; if(r==sizeof ti)ok_ti++; else {e_ti++; if(verbose&&e_ti<4)printf("taskinfo pid %d r=%d: %s\n",p,r,strerror(errno));}
  struct proc_bsdinfo bi; a=now(); r=proc_pidinfo(p,PROC_PIDTBSDINFO,0,&bi,sizeof bi); tb2+=now()-a; if(r==sizeof bi)ok_bsd++; else e_bsd++;
  char path[PROC_PIDPATHINFO_MAXSIZE]; a=now(); r=proc_pidpath(p,path,sizeof path); tp+=now()-a; if(r>0)ok_path++; else {e_path++; if(verbose&&e_path<4)printf("pidpath pid %d: %s\n",p,strerror(errno));}
  int mib[3]={CTL_KERN,KERN_PROCARGS2,p}; static char buf[1<<20]; size_t sz=sizeof buf; a=now(); r=sysctl(mib,3,buf,&sz,NULL,0); ta+=now()-a; if(r==0)ok_args++; else {e_args++; if(verbose&&e_args<4)printf("procargs2 pid %d: %s\n",p,strerror(errno));}
  a=now(); int rp=responsibility_get_pid_responsible_for_pid(p); tre+=now()-a; if(rp>0)ok_resp++;
  uint64_t co[8]={0}; a=now(); r=proc_pidinfo(p,20,0,co,sizeof co); tc+=now()-a; if(r>0)ok_co++; else e_co++;
  if(verbose&&(strstr(path,"Google Chrome Helper")||strstr(path,"Safari")) ){ printf("pid %d resp %d coal %llu/%llu path %s foot %.1fMB rss %.1fMB\n",p,rp,co[0],co[1],path,ru.ri_phys_footprint/1048576.0,ru.ri_resident_size/1048576.0);}
 }
 double tot=now()-t0;
 printf("rusage ok %d err %d | taskinfo ok %d err %d | bsdinfo ok %d err %d | pidpath ok %d err %d | procargs2 ok %d err %d | resp ok %d | coal ok %d err %d\n",ok_ru,e_ru,ok_ti,e_ti,ok_bsd,e_bsd,ok_path,e_path,ok_args,e_args,ok_resp,ok_co,e_co);
 printf("total %.2f ms (wall; timers add overhead): rusage %.2f taskinfo %.2f bsd %.2f path %.2f args %.2f resp %.2f coal %.2f\n",tot,tr,tt,tb2,tp,ta,tre,tc);
}
